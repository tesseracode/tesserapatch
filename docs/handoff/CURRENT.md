# Current Handoff

## Status

**Cluster state**: IDLE

Artifact-validation/provenance PRD rev-5 and ADR-034 rev-2 are accepted by
both reviewers. GH #10 is closed and the planning gate passes 8/8. No
implementation has begun.

## Active Task

- **Task ID**: PRD-artifact-validation-and-provenance
- **Description**: Define truthful read-only intent-artifact inspection,
  provenance/migration boundaries and `tpatch prepare --check` as the
  prerequisite to mutating preparation.
- **Status**: Complete
- **Assigned**: 2026-08-13
- **WAVE_BASE**: `0aa0d956b090288780b51d8270eb3a250fabeee3`
- **Rev-1 writer base**: `3ecfa38`
- **Rev-2 writer base**: `c590f17`
- **Rev-3 writer base**: `5a678b5`
- **Rev-4 writer base**: `be33d2a`
- **Issue**: [GH #10](https://github.com/tesseracode/tesserapatch/issues/10)
- **Scope**: one PRD + one ADR + ADR index + handoff.
- **Release tag**: v0.15.1 remains fixed at `15560af`
- **Accepted contract tip**: `cd15165`
- **Close tracking commit**: `cb771ce`

## Final Verdict

- **Internal**: APPROVED at `cd15165`.
- **External**: APPROVED at `0275067`; final byline/guard folds changed no
  decision or matrix shape.
- **Accepted contract**: PRD rev-5, 208 AVP rows.
- **Accepted architecture**: ADR-034 rev-2, D1–D18.
- **Claims**: 95 repository + 24 Go-stdlib claims.
- **Implementation**: none.
- **Provenance ADR trigger**: still unfired; `unknown` remains constant.
- **Prepare PRD**: may now enter planning, but mutating implementation remains
  unauthorized until that separate PRD is accepted.

## PRD Writer Contract

- Claims-audit current `manual.go`, Path A workflow, `FeatureStatus.Notes`,
  `next`, `cycle`, feature layout and shipped skill surfaces.
- Define deterministic structural artifact states without claiming semantic
  quality.
- Define read-only `prepare --check` human/JSON output, precedence and exits.
- Pin `provenance: unknown` until accepted metadata proves otherwise.
- Decide explicitly whether individual mutating `--manual` gates change.
- Preserve existing `defined`, `next` and `cycle` behavior unless a behavior
  delta is enumerated and accepted.
- Cover symlinks, non-regular files, unreadable paths, privacy, determinism,
  migration and Path A/B parity.
- Evaluate provenance representation alternatives. No ADR is pre-created; if
  the PRD selects persistent representation, flag the ADR gate before
  acceptance for implementation.
- Exclude mutating prepare, regenerate, provider execution, atomic bundle
  publication, new lifecycle state and downstream SDD mandates.
- Mark WP-005 Graduated when the PRD file is created.

## PRD Writer Result — rev-0 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | **new**, 1,472 lines, `Status: Draft — Awaiting Review (rev-0)` |
| `docs/whitepapers/WP-005-spec-driven-workflows.md` | header only: `Exploring` → `Graduated (2026-08-13)` + one `Related` bullet to the PRD. Body and turn log untouched. |
| `docs/whitepapers/README.md` | index row: WP-005 `Exploring` → `Graduated (2026-08-13)` |
| `docs/handoff/CURRENT.md` | this section + `Cluster state` flip |

Nothing else was touched. No edit to `docs/ROADMAP.md`, `docs/supervisor/LOG.md`,
`SPEC.md`, `docs/adrs/**`, `internal/**`, `cmd/**`, `assets/**`, `tests/**`,
`.wave-close-allowlist`, or any guarded untracked WIP (`WP-004`, `WP-006`,
`WP-007`, `PRD-recurring-patches.md`, state-of-the-art case studies).

### Counts

- **64 claims** in the §23 claims-audit appendix, each with a `file:line`
  anchor.
- **95 acceptance rows** (`AVP-001`…`AVP-095`) across 12 categories: CLI
  grammar 10, classification 20, readiness/exit 8, output shape 14, zero
  mutation 6, provenance 5, compatibility 9, Path A/B parity 4,
  security/privacy 6, concurrency 4, source scans/parity 6, totality guards 3.
- **133 distinct `file:line` anchors** across the whole document; all
  mechanically verified in-range at HEAD `12980f2`, and the load-bearing ones
  content-verified by reading the cited lines.

### Contract decisions made (not deferred)

1. **Scope**: `tpatch prepare <slug> --check [--json] [--quiet] [--path]` only.
   Plain `prepare` refuses with exit 4 *before* the store is opened;
   `--manual` / `--regenerate` are deliberately **unregistered** (cobra
   unknown-flag → exit 1).
2. **Turn-4 question answered**: mutating `analyze|define|explore|implement
   --manual` gates are **unchanged** in this slice. The inspector is pure and
   wired to `prepare --check` only. Six acceptance rows (`AVP-064`…`AVP-069`)
   pin the *loose* current behavior — including "zero-byte `spec.md` still
   advances to `defined`" — as deliberately unchanged. §12.3 lists what a
   future tightening PRD must enumerate.
3. **Closed 9-value state enum** extending the shipped
   `absent`/`present-empty`/`present-nonempty` vocabulary with
   `symlink-refused`, `not-regular`, `unreadable`, `oversize`,
   `invalid-structured`, `unstable`; 17-row total precedence table.
   `unstable` deliberately outranks every content-derived state so a
   mid-truncation read is never reported as `present-empty`.
4. **Required = `analysis.md` + `spec.md`.** `exploration.md` and
   `artifacts/analysis.json` are optional and can never change readiness; the
   sidecar is reported separately, and no output field may assert Path A vs
   Path B for a feature (that would be provenance inference from sidecar
   presence).
5. **Provenance = constant `unknown`.** Seven forbidden inference sources
   enumerated with anchors. Four persistence alternatives evaluated (status
   sub-record / dedicated manifest / derivation-only / write-time attestation);
   **none selected in rev-0** — the PRD is coherent without one — and the ADR
   trigger is stated as a gate on any future PRD that selects one.
6. **Exit envelope**: 0 ready / 1 generic CLI / 2 not_ready / 3 indeterminate
   (abort or required-artifact instability) / 4 reserved-surface refusal, with
   a six-step first-match-wins precedence and five closed abort codes.
7. **Output**: `schema_version: 1`, fixed key order, fixed-length 4-element
   `artifacts` array (even on abort), closed reason/advisory/abort catalogs,
   frozen disclaimer string, verify-style `--json`/`--quiet` stream routing.
   No wall-clock, no size, no hash, no content, no absolute path, no symlink
   target — 17 forbidden field names asserted absent.
8. **Snapshot semantics**: one descriptor-scoped capture per artifact, three
   named instability probes, **explicitly stated limits** (a same-length
   in-place rewrite is not detectable), no retry, no lock, and no
   cross-artifact atomicity claim.
9. **Skill parity**: v1 ships the command in all six surfaces and extends
   `requiredCommands` + `requiredAnchors` (`AVP-090`…`AVP-092`).
10. **Blocked-PRD statement**: `PRD-prepare-intent-bundle.md` remains blocked
    until this PRD is accepted — stated in the header, the summary and §20.

### Open decisions left to review (§21, four)

Q1 a later `--all` sweep mode; Q2 whether exit `4` becomes a cross-command
convention or stays local to `prepare`; Q3 the `MaxArtifactBytes = 4 MiB`
value; Q4 whether `request.md` becomes a fifth optional reported row. All four
have a stated default and none blocks review.

### Implementation status

**None.** No Go file, test, asset or CLI surface was created or modified. The
PRD defines five ordered implementation slices (S1–S5) for a future wave and
explicitly requires the AGENTS.md parallel-implementer file partition to be
declared at dispatch.

### Validation performed (docs-only change)

- Mechanical anchor audit: 133 distinct `file:line` citations parsed and
  checked in-range against the working tree; **zero** out-of-range, zero
  missing files. Re-run after the WP-005 header edit shifted that file by +2
  lines, and the ten affected anchors were re-resolved and content-verified.
- Markdown hygiene: fence balance (16 markers, balanced), table column
  consistency (0 mismatched rows), zero trailing-whitespace lines, all 13
  relative links resolve on disk.
- The `json` example block parses as valid JSON.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Notes for the reviewer

- The PRD deliberately does **not** claim a test proves semantics merely by
  existing: §18.1 states five disqualifying assertion shapes, and §18.14
  requires sensitivity regressions for the three mechanical guards
  (`AVP-093`/`094`/`095`) because a byte-scanning guard can false-pass — the
  lesson from the F2 finding recorded above in this file.
- §4 is the required WP-005 existing-primitives pre-flight and covers
  `--manual`, `cycle`, `next`, `verify`, `status`, `doctor` and the generic
  path/fs helpers, each with an anchor.
- The `apply --mode prepare` name collision is treated as a real risk with two
  mandated help-text mitigations and two acceptance rows, not waved away.

## Rev-0 Review Adjudication

- **Internal**: NEEDS REVISION (3 HIGH, 2 MEDIUM).
- **External**: APPROVED WITH NOTES; F1–F4 block implementation dispatch and
  F5–F13 are valid completeness notes.
- **Supervisor verdict**: NEEDS REVISION.

### Rev-1 required corrections

1. **CLI/output envelope**: compose quiet and all nonzero paths with the root
   error printer; define abort `feature_state`, overall/readiness and artifact
   shape; keep refusal help self-contained.
2. **Safe bounded capture**: canonical slug validation before path use;
   race-safe no-follow/nonblocking open + post-open fstat; size checks before
   and after open; bounded `Max+1` reads; total fstat/read failure rows.
3. **Readiness truth**: because `prepare` is optional, its `ready` verdict
   requires all three canonical Markdown artifacts. The analysis sidecar stays
   optional. Do not conflate optional command adoption with bundle
   completeness.
4. **Total diagnostics**: advisory selection must be state-accurate; a missing
   `status.json` continues with `feature_state: unknown`; unsafe slugs and
   every abort population have closed codes.
5. **Compatibility/acceptance bite**: add composite loose-`--manual` versus
   strict-check regressions, reverse call-graph routing guards, sidecar
   instability, growth/FIFO/path traversal and quiet-abort rows; scope
   forbidden-field guards to keys/labels rather than substrings.
6. **Provenance/docs**: define `unknown` as the stable "not provable" meaning,
   add the future `agent-as-provider` correction, and keep the ADR trigger
   bounded.

## PRD Writer Result — rev-1 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | rewritten in place; `Status: Draft — Awaiting Review (rev-1)`; a `Revision history` table records what changed vs rev-0 |
| `docs/handoff/CURRENT.md` | `Cluster state` flip to `AWAITING REVIEW`, Active Task block, and this section |

Nothing else was touched. **No** edit to `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`, `SPEC.md`, `docs/adrs/**`, `docs/whitepapers/**`
(WP-005 was already marked Graduated in rev-0 and needed no further change),
`docs/prds/PRD-prepare-intent-bundle.md` (still undrafted and still blocked),
`internal/**`, `cmd/**`, `assets/**`, `tests/**`, `.wave-close-allowlist`, or
any guarded untracked WIP (`WP-004`, `WP-006`, `WP-007`,
`PRD-recurring-patches.md`, the state-of-the-art case studies).

### Counts (mechanically verified, commit-independent)

- **2,233 lines** in the PRD (rev-0: 1,478).
- **75 claims** in the §23 claims-audit appendix (rev-0: 64), each with a
  `file:line` anchor; `C1`…`C75` contiguous with no gaps.
- **140 acceptance rows** `AVP-001`…`AVP-140` (rev-0: 95), contiguous, no
  duplicates, no retired/struck rows. Nineteen categories:
  A 10, B 20, C 8, D 14, E 6, F 5, G 9, H 4, I 6, J 4, K 6, L 3, M 6, N 5,
  O 12, P 10, Q 1, R 9, S 2.
- **By kind**: `U` 38, `I` 80, `S` 6, `G` 13, plus 3 combined-kind rows
  (`S+G`, `S+I`, `I+G`). 15 rows carry a guard component and are covered by
  the §18.21 sensitivity requirement.
- **161 distinct `file:line` anchors** across 33 files; all mechanically
  verified in-range against the working tree, zero out-of-range, zero missing
  files. Four rev-0 anchors were re-resolved after content verification
  (`phase2.go` `fileExistsAt`, `assets_test.go` skill-file/anchor blocks,
  `workflow.go` heuristic block, `verify.go` stream routing).
- **45 new rows** (`AVP-096`…`AVP-140`); every rev-0 ID kept its number, and
  the rows whose meaning changed (`AVP-031`…`AVP-036`, `AVP-042`, `AVP-048`,
  `AVP-051`, `AVP-069`, `AVP-071`, `AVP-073`, `AVP-075`, `AVP-092`,
  `AVP-095`) were amended in place per §18.1's no-renumbering rule.
- **All 140 rows are assigned to exactly one implementation slice** (S1–S5);
  zero unassigned, zero double-assigned.

### Contract decisions in rev-1 (what changed, and why)

1. **Readiness = the full intent bundle.** `ready` now requires all three
   canonical Markdown artifacts present-and-non-empty. `artifacts/analysis.json`
   stays optional and can never affect readiness. §6.2.1 gives four cited
   reasons (bundle unit per WP-005 Agreed item 7; the Path B operator guide
   already teaches authoring all three; alignment with `verify`'s later
   blocking `intent_files_present`; every gap still fully reported). §6.2.2
   answers the WP-005 "must not make exploration mandatory" constraint with a
   six-question table separating **optional command adoption** from **relaxed
   bundle semantics** — nothing calls the command, `defined` is still reachable
   without exploration, `next`/`cycle` are byte-identical.
2. **Output composed with the real root error printer.** `cobra.go:33-39`
   prints `error: %v` for every non-nil `RunE` error, so rev-0's "stderr empty
   on `--json --quiet`" was false for exits 2/3/4. §10.1 now tables all
   15 exit × flag combinations, §9.5 closes the `ExitCodeError.Message`
   catalog, and §10.5 gives the exact `--quiet` line for every outcome.
   Exit 0 is the only case with an empty stderr under `--json --quiet`.
3. **Abort shape rebuilt.** `artifacts` is `[]` **iff** `abort` is present —
   rev-0's four all-`absent` rows claimed an inspection that never happened.
   `feature_state` is `"unknown"` on every abort (never `""`), `overall` is
   fully specified with the schema-constant totals, and `advisories` is `[]`.
   §9.3 guarantees every abort is decided before the first per-artifact
   `Lstat`, which is what makes the two-shape rule total (AVP-127, AVP-128).
4. **Slug validated before any path is composed.** New §7.2 defines a canonical
   grammar (`^[a-z0-9]+(-[a-z0-9]+)*$`, ≤60 bytes) that provably accepts every
   `store.Slugify` output, plus a Windows reserved-device refusal. New abort
   code `slug-unsafe` (exit 3). **No stream ever echoes the raw argument**:
   JSON `slug` is `""`, the human header and the quiet line withhold it.
   Traversal, absolute, control-byte and non-ASCII arguments are all closed by
   one rule (AVP-102…AVP-106).
5. **Race-safe platform capture replaces `Lstat`→ordinary-open.** §7.4
   specifies `O_RDONLY|O_NOFOLLOW|O_NONBLOCK` on unix (no follow, no FIFO
   hang) and a real Windows equivalent (`FILE_FLAG_OPEN_REPARSE_POINT` +
   `GetFileType == FILE_TYPE_DISK`), then post-open `fstat`, `os.SameFile`
   identity, regular-file recheck, size cross-check, bounded read, and a
   post-read `fstat`. AVP-118 asserts the build-tagged sibling exists and that
   no target falls back to a bare `os.Open`.
6. **Bounded read is `io.LimitReader(f, MaxArtifactBytes+1)`.** A growth race
   can never allocate unbounded memory; growth past the cap during the read is
   `unstable`, not `oversize`, and the two are mutually exclusive by
   construction (row 8 pre-open vs row 17 post-open). AVP-112, AVP-116,
   AVP-140.
7. **Ladder rebuilt to 24 total rows.** Adds no-follow-refusal-at-open, fstat
   failure, descriptor kind change, Lstat-vs-fstat size mismatch, growth past
   the cap, byte-count disagreement, post-read fstat failure and post-read size
   change. Seven named instability probes, all with sidecar equivalents
   (AVP-107…AVP-117). §8.3 keeps the honest limits: same-length in-place
   rewrite is undetectable, and a second `fstat` on a held descriptor is a
   tautology.
8. **Advisory selection is a total state → advisory function.** Nine sidecar
   states → at most one `analysis-sidecar-*` advisory. `*-absent-*` fires only
   for `state == absent`; `present-empty` / `invalid-structured` /
   `unstable` / `symlink-refused` / `not-regular` / `unreadable` / `oversize`
   each get a truthful **neutral** message that never claims absence and never
   calls the condition a defect. `analysis-sidecar-unstable` now has real
   production coverage. rev-0's `exploration-absent-verify-requires-later` and
   `optional-artifact-unstable` are removed. AVP-119…AVP-122.
9. **`status.json` populations split three ways.** Absent → **continue** the
   full inspection with `feature_state: "unknown"` + advisory
   `feature-state-absent`, exit from readiness (justified by `ListFeatures`
   already treating such a directory as ordinary). Malformed and unreadable
   stay distinct aborts, because a present-but-broken metadata file is
   `doctor` D1's domain. AVP-123…AVP-126.
10. **Forbidden-field guard scoped to keys and labels.** AVP-051 walks JSON
    **key names** at every nesting level and compares the human surface against
    a closed label set — never a raw substring scan, which would have made
    `oversize`/`artifact-oversize` impossible. AVP-140 exercises an `oversize`
    artifact with the guard green, which is the proof the scoping is real.
11. **`unknown` provenance given a stable definition.** §11.1 fixes it as "no
    trustworthy provenance is available from an accepted source", with three
    consequences: it is not a placeholder, adding known values later is
    additive (no `schema_version: 2`), and legacy artifacts stay `unknown`
    forever rather than being backfilled by guess. The ADR trigger is kept
    **narrow** — it fires on selection of a persistent representation and on
    nothing else. No representation is selected.
12. **`agent-as-provider.md` correction made a required future edit.** §16.1
    row (b) requires the shipped sentence that presents `status.json.notes` as
    what "distinguishes Path B transitions from provider output" to be
    corrected to a *last-transition hint, not durable per-artifact provenance*.
    Claim C67 anchors the current wording.
13. **Refusal remediation made self-contained.** The exit-4 line names only
    `--check` and `tpatch prepare --help`; it cites no PRD, no `docs/` path and
    no URL (AVP-100). §5.3 also states *why* the guard is implementable before
    the store opens.
14. **Test bite added.** Composite differentials that first advance a real
    feature to `analyzed`/`defined` through the real `--manual` command with a
    zero-byte / whitespace-only / symlink artifact, then run the real
    `prepare --check` and assert the honest non-`ready` verdict with zero
    mutation (AVP-130…AVP-133). Routing non-invalidation now uses
    **pre-change-binary goldens** plus reverse call-graph and source guards
    (AVP-134…AVP-137) instead of a before/after no-op comparison. AVP-138
    composes all three headline §1 failure modes in one real CLI run.
15. **Guard sensitivity generalised.** §18.1 adds a disqualifying assertion
    shape for guards; §18.21 extends the sensitivity requirement from three
    hand-picked rows to every row carrying a guard component; AVP-139 is the
    meta-check.

### Unchanged from rev-0 (deliberately)

- Planning-only scope: no Go file, test, asset or CLI surface created or
  modified.
- `--manual` gates stay loose and unchanged (§12), now additionally pinned by
  the composite rows.
- Skill parity still ships the command in all six surfaces — with a new §16.2
  item 5 forbidding it from entering the phase-ordering or preflight sequence,
  so non-goal 8 (no downstream SDD mandate) is asserted, not assumed
  (AVP-092).
- `PRD-prepare-intent-bundle.md` remains blocked and undrafted (§20).
- No ADR created; no persistent provenance representation selected.

### Open decisions left to review (§21, six)

Q1 a later `--all` sweep mode; Q2 whether exit `4` becomes a cross-command
convention; Q3 the `MaxArtifactBytes = 4 MiB` value; Q4 whether `request.md`
becomes a fifth optional row; **Q5 (new)** whether the Windows reserved-device
refusal should be platform-conditional; **Q6 (new)** whether `--format` should
alias `--json` to match `next`. All six have a stated default; none blocks
review.

### Validation performed (docs-only change)

- **Anchor audit**: 161 distinct `file:line` citations across 33 files parsed
  and checked in-range; zero bad. Every newly added anchor was additionally
  content-verified by reading the cited lines (root printer, `exitCodeFor`,
  `readBounded`, `openNoFollow` unix/windows, `Slugify`, `AddFeature`,
  `LoadFeatureStatus` error split, `ListFeatures`, `snapshot-unstable`
  vocabulary, `pathgate` held-descriptor tautology, assets anchor blocks).
- **JSON examples**: both `json` fenced blocks parse; top-level key order
  matches §10.2's declared order in both (the abort example adds `abort`).
- **Matrix mechanics**: 140 rows, contiguous `AVP-001`…`AVP-140`, zero
  duplicates; every `AVP-NNN` referenced in prose resolves to a real row;
  category counts sum to 140; slice assignment is a partition.
- **Claims mechanics**: `C1`…`C75` contiguous, zero duplicates.
- **Markdown hygiene**: 26 fence markers (balanced), 50 tables with zero
  column-count mismatches, zero trailing-whitespace lines, all relative links
  resolve on disk.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Reviewer focus for rev-1

1. **§6.2 is the load-bearing change.** The question to test is not "is
   full-bundle readiness nicer" but "does §6.2.2 actually discharge WP-005
   Agreed item 6". The claim is that item 6 constrains the **lifecycle**, and
   that this command is outside the lifecycle in six specific, separately
   asserted ways.
2. **§10.1 must be read against `cobra.go:33-39`, not against `verify.go`
   alone.** `verify` copies the report routing; the `error:` line is the root's
   and applies on top. Any residual claim of an empty stderr on a nonzero exit
   is a defect.
3. **§7.5's ladder is the totality surface.** Check that no reachable
   filesystem condition is missing a row and that rows 8 and 17 really cannot
   both apply.
4. **§10.4 must stay a function of state, not of artifact id.** The rev-0
   defect was an advisory that could contradict its own artifact row.
5. **§9.4's three-way `status.json` split** is a deliberate asymmetry; the
   justification (absent is ordinary per `ListFeatures`, broken is `doctor`'s)
   is the thing to challenge, not the mechanics.
6. **AVP-136/AVP-137 depend on S1 capturing pre-change goldens.** §17 states
   this as a prerequisite; if it is skipped the rows silently degrade back into
   the rev-0 no-op comparison.


## PRD Writer Result — rev-2 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | rewritten in place; `Status: Draft — Awaiting Review (rev-2)`; byline `rev-2 at HEAD c590f17`; the `Revision history` table gains a rev-2 row and records rev-1's disposition as NEEDS REVISION |
| `docs/handoff/CURRENT.md` | `Cluster state` flip to `AWAITING REVIEW`, Active Task `Status`/`Rev-2 writer base`, and this section |

Nothing else was touched. **No** edit to `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`, `SPEC.md`, `docs/adrs/**`, `docs/whitepapers/**`
(WP-005 was marked Graduated in rev-0 and needed no further change),
`docs/prds/PRD-prepare-intent-bundle.md` (still undrafted, still blocked),
`internal/**`, `cmd/**`, `assets/**`, `tests/**`, `.github/**`,
`.wave-close-allowlist`, or any guarded untracked WIP (`WP-004`, `WP-006`,
`WP-007`, `PRD-recurring-patches.md`, the state-of-the-art case studies).

### Counts (mechanically verified, commit-independent)

- **3,075 lines** in the PRD (rev-1: 2,233; rev-0: 1,478).
- **188 acceptance rows** `AVP-001`…`AVP-188` (rev-1: 140), contiguous, zero
  duplicates, zero retired rows. **48 new rows**; every rev-1 ID kept its
  number.
- **24 categories**: A 10, B 20, C 8, D 14, E 6, F 5, G 9, H 4, I 6, J 4, K 6,
  L 3, M 6, N 5, O 12, P 10, Q 1, R 9, S 2, **T 12, V 17, W 5, X 6, Y 8**.
  Sum = 188. The letter `U` is deliberately skipped as a category so it cannot
  be confused with the `U` (unit) kind.
- **By kind**: `U` 57, `I` 94, `S` 6, `G` 23, `S+G` 3, `U+G` 2, `I+G` 1,
  `S+I` 2. Sum = 188.
- **Guard arithmetic (the rev-1 defect)**: a row carries a guard component
  **iff** its Kind contains `G` → 23 + 3 + 2 + 1 = **29 rows**. AVP-128
  (`S+I`) is now excluded, and AVP-139 derives the set mechanically instead of
  from a hand-list.
- **All 188 rows are assigned to exactly one slice** (S1 69, S2 27, S3 47,
  S4 38, S5 7). Verified as a partition: zero unassigned, zero double-assigned.
- **Claims audit split in two**: **88 repository claims** `C1`…`C88`
  (rev-1: 75), contiguous, anchored `file:line`; plus a new **§23.2 with 12
  Go-standard-library claims** `G1`…`G12`, anchored **by symbol** in
  `$GOROOT/src/os/...` rather than by line, each naming the acceptance row that
  verifies it at runtime.
- **159 distinct repository `file:line` anchors** across 35 files; all
  mechanically verified in-range, zero out-of-range, zero missing files.

### What rev-2 changed, finding by finding

1. **Rooted namespace (§7.3, §7.4).** One `os.OpenRoot(repoRoot)` per run,
   held for the whole inspection, passed to `Inspect`, closed once by the CLI.
   Every `Lstat` is `Root.Lstat`, every open is `Root.OpenFile`, every
   descriptor stat is `(*os.File).Stat`. `filepath.Join(repoRoot, …)`,
   `os.Stat`, `os.Lstat`, `os.Open`, `os.OpenFile` and `os.ReadFile` are all
   forbidden and source-scanned (AVP-089, AVP-144, AVP-150).
2. **Platform scope stated honestly (§7.4.1).** A three-row table over the Go
   build-tag split: `unix||wasip1` and `windows` are confined; `(js&&wasm)||plan9`
   is **not**, per the `Root` doc comment. A build-tagged
   `rootConfinementSupported` constant makes the unsupported targets abort
   `workspace-unsupported-platform` before `os.OpenRoot` is called
   (AVP-177, AVP-179).
3. **Component policy widened to reparse points.** The refusal predicate is
   `Mode()&(ModeSymlink|ModeIrregular)`, because Go maps a Windows **junction**
   (`IO_REPARSE_TAG_MOUNT_POINT`) to `ModeIrregular`, not `ModeSymlink`. A
   sensitivity fixture that tests only `ModeSymlink` lets the junction through
   and fails (AVP-146).
4. **The no-follow claim is dropped, and the race behavior is stated exactly
   (§7.4.2, §7.4.4).** `os.Root` **follows** in-root symlinks; it is a
   confinement primitive, not a no-follow primitive. On Unix `Root.OpenFile`
   already ORs `O_NOFOLLOW` internally and converts the `ELOOP` into an in-root
   *resolution*, so a caller cannot obtain a final-leaf refusal from it — the
   PRD says so and **no acceptance row claims otherwise**. What is claimed is a
   three-row table: an out-of-root raced link → `Root` refuses → `unreadable`,
   zero bytes; an in-root raced link to a *different* object → identity
   mismatch → `unstable`, **zero bytes**; an in-root raced link to the *same*
   inode → read proceeds, and the same-identity alias is explicitly **not**
   claimed detectable. AVP-148, AVP-149, AVP-151, plus AVP-152, a mechanical
   over-claim guard over every shipped string.
5. **Windows contract rebuilt (§7.4.3).** rev-1's raw `syscall.CreateFile` with
   `FILE_FLAG_OPEN_REPARSE_POINT` was self-contradictory — the flag makes the
   open *succeed* while the ladder classified from an open *error*. rev-2 has
   no raw syscall at all: reparse points are refused pre-open by `Root.Lstat`
   (whose Windows implementation is itself an `OPEN_REPARSE_POINT` handle open
   plus a handle-derived stat), and pipes/char devices are refused by
   `statHandle`'s `GetFileType`-derived modes at ladder rows 7 and 14.
   `openFlags()` returns `0` on Windows and `O_NOFOLLOW|O_NONBLOCK` on Unix
   (AVP-118, AVP-180).
6. **`os.SameFile`-on-a-pathname removed.** Both sides of the identity check
   are handle-derived on both platform classes. On Windows,
   `newFileStatFromGetFileInformationByHandle` clears the struct's `path` field
   specifically so `os.SameFile` will not re-fetch by pathname, and
   `(*File).Stat` is `statHandle(name, handle)` — cited as G7/G8 and asserted
   natively by AVP-176.
7. **Native Windows CI is an acceptance obligation, not an assumption.** The CI
   matrix is Linux + macOS today (C84). §16.1 makes adding `windows-latest` a
   **required** file change, §17 lands it in **S1** alongside the Windows code,
   AVP-175 parses the workflow and fails if it is absent, and AVP-176 is the
   native behavioral row. R7's severity was raised to High.
8. **Fixed-buffer read (§7.4.5).** rev-1's `io.ReadAll(io.LimitReader(f,
   Max+1))` "exact allocation ceiling" claim was **false** — `io.ReadAll` grows
   by `append`. rev-2 uses one `make([]byte, MaxArtifactBytes+1)` plus
   `io.ReadFull`, with a total four-way EOF table (`io.EOF` → empty;
   `ErrUnexpectedEOF` → captured `buf[:n]`; `nil` → grew past the cap →
   `unstable`; anything else → `unreadable`). An allocation-counting fixture
   asserts exactly one allocation of exactly `Max+1` bytes per capture
   (AVP-170…AVP-174); `io.ReadAll`, `io.LimitReader` and `os.ReadFile` are
   source-forbidden with a sensitivity fixture that reintroduces the rev-1 form.
9. **`status.json` (§9.4).** `store.LoadFeatureStatus` — `os.ReadFile` on an
   absolute pathname, symlink-following, unbounded, no kind or identity check —
   is **forbidden** (AVP-150), and the rev-1 `StateReader` seam is deleted from
   §7.1. The status file gets the full rooted discipline, its **own**
   `MaxStatusBytes = 1 MiB` cap (justified, and separate so widening Q3 cannot
   silently widen it), a 19-row first-match ladder, and a **nine-value** outcome
   enum. Two outcomes continue (`ok`, `absent`); seven abort.
10. **Closed abort catalog grown to thirteen codes (§9.4.4)** with **exact
    message templates (§9.4.5)**: `slug-unsafe`,
    `workspace-unsupported-platform`, `workspace-not-initialized`,
    `workspace-root-unopenable`, `feature-dir-unsafe`, `feature-not-found`,
    `status-symlink-refused`, `status-not-regular`, `status-oversize`,
    `status-unreadable`, `status-unstable`, `status-malformed`,
    `status-invalid-state`. Bijection code↔message asserted; no template wraps
    an `os` error or carries an absolute path (AVP-181).
11. **`FeatureState` validated before echo (§9.4.2 row 18).** The inspector
    carries its own closed twelve-value list with a two-way AST parity guard
    against `store` (AVP-165); a parsed-but-unrecognised state aborts
    `status-invalid-state` and **the offending value is never echoed**
    (AVP-164, AVP-185).
12. **Lifecycle line made truthful per population (§10.5.1).** A fifteen-row
    table (status `ok`, status `absent`, thirteen aborts). rev-1 printed
    `(status.json was not read)` on every abort, which is false for
    `status-malformed`, `status-invalid-state`, `status-unreadable` and
    `status-unstable`. AVP-154 asserts those four do **not** contain the
    substring `was not read`; AVP-153 is the totality guard.
13. **`--path` exit ownership corrected (§9.2).** `--path` is a persistent
    **string** flag (`internal/cli/cobra.go:66`); pflag validates nothing, so
    the failure surfaces inside `RunE` at `store.FindProjectRoot`
    (`internal/store/store.go:23-40`) — the actual trigger — and is bound to
    abort `workspace-not-initialized`, **exit 3**, not cobra exit 1. The
    genuine exit-1 population is unknown flags, wrong arity, and `--path` with
    no value (AVP-183, AVP-184). `store.Open` is documented as **not** the
    trigger and is never called.
14. **Quiet abort distinguishes codes.** All thirteen `--quiet` abort lines are
    pairwise distinct and carry the same closed token as `abort.code` and the
    `error:` line; the bare `— indeterminate` form is reserved for the one
    non-abort indeterminate case (AVP-098, AVP-184).
15. **"Printable ASCII" replaced (§14.3).** That claim was wrong on the
    command's own happy path — the `—` in the quiet line and the `→`
    remediation marker are required non-ASCII house style. The rule is now: no
    ASCII control byte other than the renderer's own `0x0A` (so no `0x1B`, no
    tab, no CR), no attacker-argument bytes, valid UTF-8, house style preserved
    (AVP-187).
16. **`slug-unsafe` remediation de-looped (§7.2).** rev-1 said "Run `tpatch
    status` to list valid slugs" — but `ListFeatures` applies no canonicality
    filter (C16), so `status` prints the same refused name back. The message
    now names `tpatch add` and the rename path only. §13.3 splits the
    hand-assembled population into a canonically-named row (fully inspectable,
    the only claim made) and a non-canonically-named row (AVP-186).
17. **Skill wording for exit 2 (§16.2 item 6).** Verbatim required paragraph in
    all six surfaces stating exit 2 is an expected report outcome, not a
    workflow or system failure; it may not be called an error, a failure or a
    blocker. AVP-188, with a sensitivity fixture on "fails with exit 2". New
    risk R12.
18. **Source claims corrected.** C60 now records that
    `internal/rescap/pathopen_windows.go` is an **unsupported compile-only
    stub** (bare `os.OpenFile`, `isSymlinkLoopError` always false) — precedent
    for the problem, not a reusable implementation. C61/C82 record that
    `rescap.readBounded` uses a growable `append` buffer. C28's stale
    `types.go:207-215` anchor for the `DependsOn` doc comment is corrected to
    `:219-234`. §4's preflight gains rows for `store.LoadFeatureStatus` and
    `os.Root`.
19. **New ladder rows and probes.** The per-artifact ladder stays 24 rows but
    is rebuilt on `Root`; row 10 (out-of-root escape refusal → `unreadable`) is
    stated separately from row 11 for honesty even though both share a state,
    because `errPathEscapes` is unexported and undiscriminable (G11). The seven
    instability probes move to rows 9, 13, 14, 15, 17, 18, 20.
20. **New alternatives recorded (§22).** Seven new rejected-alternative rows,
    each naming a rev-1 behavior and why it is gone:
    `io.ReadAll(LimitReader)`, pathname-walk resolution, the raw `CreateFile`,
    `os.SameFile`-on-a-pathname, `store.LoadFeatureStatus`, grouping the seven
    status aborts into one code, and the printable-ASCII assertion.

### Unchanged from rev-1 (deliberately, as instructed)

- **No mutating `prepare`**; plain `prepare <slug>` still refuses with exit 4
  before anything runs.
- **No lifecycle state**, no new `FeatureState`, no transition, no write.
- **Individual `--manual` / `next` / `cycle` unchanged** (§12, §13.2), still
  pinned by AVP-064…AVP-069 and the composite rows AVP-130…AVP-133.
- **`provenance` is the constant `unknown`** with §11.1's stable "not provable"
  meaning; the seven forbidden inference sources are unchanged.
- **No persistent provenance representation selected; no ADR created.** The
  §11.4 trigger remains narrow and unexercised.
- **`PRD-prepare-intent-bundle.md` remains blocked and undrafted** (§20).
- Full-bundle readiness (§6.2), the advisory total function (§10.4), the
  two-shape `artifacts` iff-`abort` rule, the composite differentials, the
  reverse call-graph routing guards, the sidecar instability rows and the
  forbidden-field key-name scoping all carry over intact.

### Open decisions left to review (§21, eight)

Q1 `--all`; Q2 whether exit `4` becomes a cross-command convention; Q3
`MaxArtifactBytes = 4 MiB`; Q4 `request.md` as a fifth optional row; Q5
platform-conditional Windows reserved-device refusal; Q6 `--format` aliasing;
**Q7 (new)** `MaxStatusBytes = 1 MiB` and whether it stays separate from
`MaxArtifactBytes`; **Q8 (new)** whether `workspace-unsupported-platform`
should be a compile-time refusal instead of a runtime abort. All eight have a
stated default; none blocks review.

### Implementation status

**None.** Planning only. No Go file, test, asset, workflow or CLI surface was
created or modified. The PRD defines five ordered slices (S1–S5); S1 now also
carries the `windows-latest` CI matrix row and the pre-change routing goldens.

### Validation performed (docs-only change)

- **Repository anchors**: 159 distinct `file:line` citations across 35 files
  parsed and checked in-range; zero bad. Every new or changed anchor was
  additionally content-verified by reading the cited lines — `FindProjectRoot`,
  `ListFeatures`, `ValidFeatureState`, the `FeatureState` const block, the
  `--path` registration, `LoadFeatureStatus`, `SaveFeatureStatus`,
  `AddFeature`, `Notes`, `DependsOn`, the CI matrix and step block, `go.mod`'s
  Go version, the `rescap` Windows stub, `readBounded`, and
  `pathgate.SamePathIdentity`.
- **Go stdlib claims**: read directly from the pinned toolchain's `GOROOT`
  (`go1.26.5`) — `os/root.go`, `root_openat.go`, `root_unix.go`,
  `root_windows.go`, `root_noopenat.go`, `stat_windows.go`, `stat_unix.go`,
  `types_windows.go`, `types_unix.go`, `file.go`. Cited **by symbol**, never by
  line, and every one is additionally backed by a runtime acceptance row.
- **Matrix mechanics**: 188 rows, contiguous `AVP-001`…`AVP-188`, zero
  duplicates; every `AVP-NNN` referenced in prose resolves to a real row;
  category counts sum to 188; kind counts sum to 188; the slice assignment is a
  verified partition of 1…188.
- **Claims mechanics**: `C1`…`C88` and `G1`…`G12` both contiguous, zero
  duplicates.
- **Markdown hygiene**: 30 fence markers (balanced); every table's data rows
  match its header column count (checked with `\|`-escape awareness); zero
  trailing-whitespace lines; all 13 relative links resolve on disk; both `json`
  fenced blocks parse.
- **Cross-reference closure**: every `§N.N` reference in the document resolves
  to a heading that exists.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Reviewer focus for rev-2

1. **§7.4.2 and §7.4.4 are the load-bearing honesty sections.** The question to
   test is not "is `os.Root` safer" but "does the PRD claim exactly what
   `os.Root` provides and nothing more". Specifically: it claims **confinement**
   and **no substitution**, and it explicitly disclaims final-leaf no-follow
   and same-identity-alias detection. AVP-152 is the mechanical guard against
   any shipped string drifting back into the stronger claim.
2. **The Unix `O_NOFOLLOW` flag is deliberately described as inert.**
   `rootOpenFileNolog` already sets it and `doInRoot` converts the signal into
   an in-root resolution. If a reviewer finds any row or sentence that treats
   it as a refusal mechanism, that is a defect.
3. **§9.4.2's status ladder must be checked for totality against §9.4.3's nine
   outcomes and §9.4.4's thirteen codes.** The three tables have to agree, and
   AVP-168 is the guard that says so.
4. **§10.5.1 is the fix for the falsest sentence in rev-1.** Check that no
   annotation says "was not read" for a population where a read happened.
5. **§16.1's CI row is a hard requirement, not advice.** If the reviewer thinks
   `GOOS=windows` cross-building is sufficient, that disagreement should be
   raised now — the whole Windows half of §7.4.3 rests on runtime behavior a
   cross-build cannot execute.
6. **§18.26's guard arithmetic is stated as a predicate.** Recount it: 23 pure
   `G` + 3 `S+G` + 2 `U+G` + 1 `I+G` = 29, and AVP-128 (`S+I`) is out. If the
   count does not reproduce, the section is wrong.
7. **The claims audit is now two tables with two anchoring conventions**
   (§23.1 `file:line`, §23.2 by symbol). The by-symbol choice is deliberate —
   toolchain line numbers drift across patch releases — and each G-claim names
   its runtime verifier so the citation is not the only evidence.

## PRD Writer Result — rev-3 + ADR-034 rev-0 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | edited in place; `Status: Draft — Awaiting Review (rev-3)`; byline `rev-3 at HEAD 5a678b5`; new `Architecture` header line linking ADR-034; `Related` gains an ADR-034 bullet; `Revision history` gains a rev-3 row and records rev-2's disposition as NEEDS REVISION. **3,830 lines** (rev-2: 3,075). |
| `docs/adrs/ADR-034-rooted-filesystem-inspection-boundary.md` | **new**, 635 lines, `Status: Proposed — Awaiting Review (rev-0)`, fourteen decisions D1–D14 |
| `docs/adrs/README.md` | one index line added for ADR-034 (Proposed — Awaiting Review), nothing else touched |
| `docs/handoff/CURRENT.md` | `Cluster state` flip to `AWAITING REVIEW`, Active Task `Status`/`Rev-3 writer base`/`Scope`, and this section |

Nothing else was touched. **No** edit to `docs/ROADMAP.md`,
`docs/supervisor/LOG.md`, `SPEC.md`, `docs/whitepapers/**` (WP-005 was marked
Graduated in rev-0 and needed no further change),
`docs/prds/PRD-prepare-intent-bundle.md` (still undrafted, still blocked), any
other `docs/adrs/ADR-0*.md`, `internal/**`, `cmd/**`, `assets/**`, `tests/**`,
`.github/**`, `.wave-close-allowlist`, or any guarded untracked WIP (`WP-004`,
`WP-006`, `WP-007`, `PRD-recurring-patches.md`, the state-of-the-art case
studies).

### Counts (mechanically verified, commit-independent)

- **PRD 3,830 lines** (rev-2: 3,075; rev-1: 2,233; rev-0: 1,478);
  **ADR 635 lines**.
- **202 acceptance rows** `AVP-001`…`AVP-202` (rev-2: 188), contiguous, zero
  duplicates, zero retired rows. **14 new rows**; every rev-2 ID kept its
  number, and the rows whose meaning changed (`AVP-083`, `AVP-108`, `AVP-116`,
  `AVP-117`, `AVP-118`, `AVP-139`, `AVP-144`, `AVP-146`, `AVP-152`, `AVP-167`,
  `AVP-170`, `AVP-176`, `AVP-177`, `AVP-182`, `AVP-184`, `AVP-187`) were
  amended in place per §18.1's no-renumbering rule.
- **25 categories**: A 10, B 20, C 8, D 14, E 6, F 5, G 9, H 4, I 6, J 4, K 6,
  L 3, M 6, N 5, O 12, P 10, Q 1, R 9, S 2, T 12, V 17, W 5, X 6, Y 8,
  **Z 14**. Sum = 202, verified by parsing the section headings rather than by
  hand.
- **By kind**: `U` 59, `I` 96, `S` 6, `G` 30, `S+G` 4, `U+G` 4, `I+G` 1,
  `S+I` 2. Sum = 202.
- **Guard arithmetic**: Kind contains `G` → 30 + 4 + 4 + 1 = **39 rows**
  (rev-2: 29). Complement 59 + 96 + 6 + 2 = 163; 39 + 163 = 202.
- **Slice partition verified programmatically**: S1 78, S2 28, S3 49, S4 38,
  S5 9 = 202; zero unassigned, zero double-assigned.
- **Claims audit**: **92 repository claims** `C1`…`C92` (rev-2: 88) and
  **21 Go-stdlib claims** `G1`…`G19` plus `G5a`/`G9a` (rev-2: 12), now split
  into **8 `contract`** and **13 `tripwire`** rows.
- **164 distinct repository `file:line` anchors** across 36 files; all
  mechanically verified in-range, zero out-of-range, zero missing files.
- **ADR-034**: 14 decisions, 10 rejected alternatives, a 14-row
  decision→acceptance-row dependency table whose every `AVP` reference resolves
  in the PRD.

### What rev-3 changed, finding by finding

**os.Root scope / ADR-034**

1. **ADR-034 created** (`Proposed — Awaiting Review`, rev-0) and linked from the
   PRD header, `Related` list and eleven inline decision points. The ADR is
   declared normative where the two overlap. Its four alternatives are the ones
   the review named: keep the `rescap` pathname `GatePath`; hand-roll
   `openat`/`NtCreateFile`; `os.Root`; platform refusal — with `os.Root` chosen
   for this read-only inspector and the reasoning stated in full.
2. **Logical, not physical, confinement (§7.4.2, ADR D2).** The `Root` doc
   comment's own sentence — "Methods on `Root` do not prohibit traversal of
   filesystem boundaries, Linux bind mounts, /proc special files, or access to
   Unix device files" — is now quoted verbatim (G13) and is the basis of the
   scope statement. Three rev-2 sentences are **prohibited by name**, including
   "no path outside the repository is ever opened, read, or named". A table
   states what actually closes each leaf shape: the kind gate and the bounded
   read, not the root. The PRD explicitly does **not** claim the bytes it reads
   physically originate inside the repository's filesystem.
3. **Workspace discovery declared outside the rooted capture (§7.3 step 1,
   §9.3 step 5, §14.1, ADR D3).** rev-2's "every filesystem operation it
   performs goes through one `*os.Root`" is corrected to "every filesystem
   operation **after workspace discovery**". `FindProjectRoot` is an ordinary
   symlink-following pathname walk that runs before `os.OpenRoot` — it produces
   the directory the root is opened on — and `--path` is stated as trusted
   input.
4. **`fs.ValidPath` replaces `EnsureSafeRepoPath` entirely (§7.3 step 2, ADR
   D4).** The §4 preflight row, the §14.1 summary and a new §22 alternatives
   row all record *why*: it is a lexical test against an absolute repository
   prefix, applied to a root-relative name in a handle-based design, and under
   `--path` from an unrelated cwd there is no prefix for it to test. New claim
   C91 anchors the signature. AVP-144 asserts both the replacement and the
   removal.
5. **Fail-closed platform allowlist (§7.4.1, ADR D5).** `//go:build unix ||
   windows || wasip1` → `true`, its exact negation → `false`. The `true`
   expression is byte-identical to `$GOROOT/src/os/root_openat.go`'s own tag
   and AVP-191 compares the two texts. `js/wasm`, `plan9` **and any future
   unmatched `GOOS`** abort `workspace-unsupported-platform`. The rev-2
   denylist is recorded as a rejected alternative with the fail-open failure
   mode spelled out.
6. **Slug validated before the platform check (§7.2, §9.3, ADR D5).**
   Precedence is now 1 cobra, 2 reserved-surface, **3 slug**, **4 platform**,
   5 discovery, 6 root open, 7 feature walk, 8 status, 9 instability,
   10 shortfall, 11 zero — which is what keeps §10.2 rule 8's "`slug` is `""`
   **iff** `slug-unsafe`" biconditional total on an unsupported target.
   AVP-192.
7. **rescap migration locked OUT OF SCOPE (ADR D11).** New claim C92; a new §22
   alternatives row; the ADR states that a wave which "helpfully" migrates
   `rescap` has exceeded scope.

**Symlink / identity / Windows**

8. **Pre- *and* post-component walks per capture (§7.4.4 steps 1 and 11, ladder
   rows 20a/20b, ADR D8).** A component observed as changed after the read
   makes the artifact `unstable`, bytes discarded, no content state — even when
   the leaf identity matched. Rows are lettered, not renumbered, so every rev-2
   row-number citation still resolves. AVP-195.
9. **Identity promise weakened to "an object *observed as different* is never
   read" (§7.4.4, §8.3, ADR D8).** Five divergences between "different" and
   "observed as different" are enumerated: same-length rewrite, same-identity
   alias, **hard-link alias** (undetectable by construction), **inode/NTFS
   file-ID reuse**, and **swap-and-restore between probes** — plus the
   walk→`Lstat`→open window, stated as a genuine unclosed TOCTOU residue.
   §8.3 now lists six limits (rev-2: three). AVP-196 pins three of them as
   *limits*, not capabilities.
10. **Windows reparse mapping corrected (§7.4.3, G9/G9a, ADR D7).** rev-2's
    "every other reparse tag sets `ModeIrregular`" was false: `AF_UNIX` sets
    **`ModeSocket`** and `DEDUP` sets **no type bit at all** (Go deliberately
    treats it as regular, with an explanatory comment). A five-row tag table
    states symlink → `ModeSymlink`; junction/`MOUNT_POINT` → name surrogate,
    `ModeDir` and `GetFileType` bits suppressed, `ModeIrregular` via `default`;
    other tags → `ModeIrregular` **only** in the default branch. The
    `ModeSymlink|ModeIrregular` predicate is restated as a *refusal* predicate
    that is necessary and **not sufficient**, backed by the `!IsRegular()` kind
    gate. AVP-198.
11. **`//go:debug winsymlink=1` pinned in `package main`** (`cmd/tpatch/main.go`,
    new claim C89) as a required implementation and acceptance obligation, with
    both a source half and a native behavioral half in AVP-198, plus the honest
    limit that the `GODEBUG` env var still overrides a `//go:debug` default.
    G14 and G15 anchor the fallback path and the setting's history.
12. **Identity mechanism labelled a Go-version tripwire (§7.4.4, ADR D10).**
    `Root.Lstat` + opened `File.Stat` + `os.SameFile` under the Go 1.26 pin;
    G7/G8/G16 are **tripwire**-classified; AVP-176 is described as an
    upgrade tripwire, not an eternal public guarantee. No ordinary `os.Lstat`
    or pathname reopening appears anywhere in the design.
13. **`O_NOFOLLOW` removed from the caller's flags (§7.4.3, ADR D6).** rev-2
    passed it "belt-and-braces" while documenting that `Root` consumes it —
    a false affordance. `openFlags()` is now exactly `syscall.O_NONBLOCK` on
    non-Windows and exactly `0` on Windows; AVP-118 asserts the flag set and
    fails if `O_NOFOLLOW` returns. A stable FIFO is refused pre-open at ladder
    row 7; a raced FIFO returns without hanging under `O_NONBLOCK` and is
    caught by the row-14 kind recheck.
14. **`O_NONBLOCK` pass-through recorded as an implementation tripwire**
    (G17, ADR D10) with **AVP-200**, a real-FIFO-under-deadline Go-upgrade
    test whose stated job is to go red at upgrade time rather than let the
    command hang in the field.
15. **Native `windows-latest` mandatory; junctions must FAIL not SKIP
    (§16.1, ADR D13, AVP-199).** `cmd /c mklink /J` in test code (no privilege
    or Developer Mode needed, unlike `mklink /D`), `t.Fatal` on unavailability,
    and a source half forbidding `t.Skip`/`t.Skipf`/`t.SkipNow` in the fixture
    path — the only permitted guard is `runtime.GOOS != "windows"`. The CI
    change remains a future implementation obligation of S1.

**Fixed buffer / test seams**

16. **One reused scratch buffer per invocation (§7.4.5, ADR D9, AVP-197).**
    rev-2's one-buffer-*per-capture* had a ~20 MiB worst case it never
    totalled. rev-3 allocates exactly one `MaxArtifactBytes+1` array in the CLI
    layer and reuses it sequentially for the status capture and all four
    artifacts; the status capture takes `scratch[:MaxStatusBytes+1]`, a
    sub-slice of the same array, which makes `MaxStatusBytes <
    MaxArtifactBytes` a compile-time-asserted structural invariant. The cost is
    stated honestly as a flat **4,194,305 bytes for every invocation**,
    including aborts. `io.ReadFull` EOF semantics are unchanged and total.
    Q9 records lazy allocation as the revisable alternative.
17. **Cap↔message coupling (§7.4.5, §9.4.2, AVP-201).** `4 MiB` and `1 MiB`
    appear verbatim in frozen messages; a guard derives each unit string from
    its constant and fails in both directions.
18. **Injected operation seam defined (§7.1.1, ADR D12).** `RootOps` (2
    methods) + `FileOps` (3 methods) wrapping `Lstat`/`OpenFile` and
    `Stat`/`Read`/`Close`, with deterministic `before`/`after` hooks available
    to test implementations only. **Exactly one non-test implementation** — the
    `*os.Root`/`*os.File` adapter — asserted by AVP-194 with a sensitivity
    fixture. The source guards explicitly allow the seam and still ban
    alternate pathname readers across production *and* test files. This is what
    makes every injected `fstat`/read/race row implementable; rev-2 specified
    two dozen rows that could not be written against a concrete `*os.Root`.
    Because `RootOps` exposes no mutator and no pathname-taking method, "cannot
    mutate, cannot enumerate" becomes a type-level property.

**Status / output / CLI**

19. **Status stays under the same boundary** — same root, same pre/post
    component walks, same `openFlags()`, same identity/kind/size rechecks, same
    shared buffer, its own cap, same valid-state gate before any echo. Stated
    in §9.4.2 and locked by ADR D1 and the ADR's closing paragraph. All nine
    status populations, the thirteen abort codes, the fifteen lifecycle
    annotations and the closed abort-message templates remain total.
20. **Every drifted citation corrected**, including the message-catalog
    references that pointed at the read-primitive guard (`AVP-172` → `AVP-181`
    in four places), the status-echo reference (`AVP-161` → `AVP-166`), the
    reparse-component reference (`AVP-166` → `AVP-146`), the escape-landing
    reference (`AVP-157` → `AVP-149`), the runtime-spy reference (`AVP-160` →
    `AVP-089`/`AVP-149`), the cap references (`AVP-162`/`AVP-163` →
    `AVP-157`/`AVP-158`/`AVP-174`), the identity-soundness and same-identity
    references (`AVP-167`/`AVP-155…158`/`AVP-159` → `AVP-148`/`AVP-149`/
    `AVP-151`/`AVP-152`/`AVP-176`), the forbidden-primitive reference
    (`AVP-173` → `AVP-172`), and the `rescap` reference (`AVP-172` →
    `AVP-180`). `AVP-083`'s ladder row is corrected from 10 to 9. `AVP-167`,
    which duplicated `AVP-124`, is amended in place into the human-surface half.
21. **`--path` populations unchanged and still correctly anchored**: malformed
    or missing `--path` **value** is cobra exit 1; workspace discovery failure
    is `RunE` exit 3 with `workspace-not-initialized` (AVP-183, AVP-184).
22. **Attacker-byte guarantees scoped to command-owned output (§14.3.1,
    AVP-193).** rev-2's unqualified "no byte of stdout or stderr is a control
    character" swept in cobra's and pflag's own parse diagnostics, which are
    produced before `RunE` from raw `os.Args` and interpolate the offending
    argument verbatim. §14.3.1 enumerates what the command owns, states that
    unknown-flag and arity errors are **outside the schema and not claimed
    sanitized**, and records that a hostile slug with a well-formed flag set
    still reaches `slug-unsafe` and is still withheld. AVP-193's sensitivity
    fixture fails if a future implementation intercepts and re-renders cobra's
    error.
23. **Canonical hand-assembled scope, loop-free `slug-unsafe` remediation and
    the skill exit-2 paragraph** are carried forward unchanged from rev-2, with
    a new §16.2 item 7 forbidding any skill surface from restating an
    `os.Root` guarantee (AVP-189).

**Matrix / citations**

24. **AVP-202, the citation-and-arithmetic guard.** Asserts every prose
    `AVP-NNN` resolves to a declared row; contiguity `AVP-001`…`AVP-202`; that
    each category's stated count equals the rows under that heading; that each
    kind's count equals the rows with that exact Kind string; that the guard
    predicate reproduces the stated guard count; and that §17's slice
    assignment is a partition. Four sensitivity fixtures. **Its doc comment
    states what it cannot prove**: resolution and arithmetic are mechanical,
    semantic mapping of a citation to a sentence is not, and remains a reviewer
    obligation.
25. **Guard arithmetic restated as reproducible arithmetic**: 30 + 4 + 4 + 1 =
    39 guard rows; complement 59 + 96 + 6 + 2 = 163; 39 + 163 = 202.
26. **Nine new §22 rejected-alternative rows**, each naming a rev-2 behavior
    and why it is gone: keeping `EnsureSafeRepoPath`; the platform denylist;
    caller-side `O_NOFOLLOW`; process-wide byte assertions; per-capture
    buffers; "a different object is never read"; the single ancestor walk;
    concrete-`*os.Root` testing; and migrating `rescap`.

### Unchanged from rev-2 (deliberately, as instructed)

- **Read-only check with full three-doc readiness** (§6.2): `ready` requires
  `analysis.md` + `spec.md` + `exploration.md`; the sidecar stays optional and
  can never affect readiness.
- **No mutation, no new state**: no mutating `prepare` (exit 4 refusal), no new
  `FeatureState`, no transition, no write on any path.
- **Loose `--manual`, `next` and `cycle` unchanged** (§12, §13.2), still pinned
  by AVP-064…AVP-069 and the composite rows AVP-130…AVP-133.
- **`provenance` is the stable constant `unknown`** with §11.1's "not provable"
  meaning and the seven forbidden inference sources.
- **No provenance ADR trigger fired.** ADR-034 D14 states this explicitly and
  forbids citing ADR-034 as provenance precedent: it is a filesystem *access*
  boundary and says nothing about what may be persisted about authorship.
- **`PRD-prepare-intent-bundle.md` remains blocked and undrafted** (§20).

### Open decisions left to review (§21, ten)

Q1 `--all`; Q2 exit `4` as a cross-command convention; Q3
`MaxArtifactBytes = 4 MiB`; Q4 `request.md` as a fifth row; Q5
platform-conditional Windows reserved-device refusal; Q6 `--format` aliasing;
Q7 `MaxStatusBytes = 1 MiB` and its separateness; Q8 compile-time vs runtime
platform refusal (narrowed by rev-3's allowlist); **Q9 (new)** lazy vs
unconditional scratch allocation; **Q10 (new)** whether the `RootOps`/`FileOps`
seam should be unexported. All ten have a stated default; none blocks review.

### Implementation status

**None.** Planning only. No Go file, test, asset, workflow or CLI surface was
created or modified. `cmd/tpatch/main.go` and `.github/workflows/ci.yml` are
named as **required future** edits (§16.1) and were **not** touched.

### Validation performed (docs-only change)

- **AVP mechanics**: 202 declared rows, contiguous `AVP-001`…`AVP-202`, zero
  duplicates; **every** `AVP-NNN` token in prose resolves to a declared row
  (zero unresolved); category counts parsed from the section headings sum to
  202 and match the §18.27 table exactly; kind counts parsed from the Kind
  column sum to 202 and match; the guard predicate yields 39; **§17's slice
  assignment was expanded programmatically and verified as a partition of
  1…202** (S1 78, S2 28, S3 49, S4 38, S5 9 — zero duplicates, zero missing).
- **Citation semantics**: the full prose→row citation list was dumped with each
  cited row's actual subject and reviewed line by line; every mismatch found is
  listed in item 20 above and corrected.
- **Repository anchors**: 164 distinct `file:line` citations across 36 files
  parsed and checked in-range; zero bad. The four new/changed ones were
  content-verified by reading the cited lines — `safety.EnsureSafeRepoPath`'s
  signature and prefix test (C91), `cmd/tpatch/main.go`'s eleven directive-free
  lines (C89), the CI matrix block (C90), and the `rescap` gate/content
  citations (C92).
- **Go stdlib claims**: re-read from the pinned toolchain's `GOROOT`
  (`go1.26.5`) — `os/root.go` (incl. the filesystem-boundary paragraph and
  `rootMaxSymlinks`), `root_openat.go`/`root_noopenat.go`/`root_unix.go`/
  `root_windows.go` build tags, `root_unix.go` `rootOpenFileNolog`/
  `checkSymlink`, `root_windows.go` `rootStat`, `stat_windows.go` `statHandle`
  and `(*File).Stat`, `types_windows.go` `mode`/`Mode`/`modePreGo1_23`/
  `isReparseTagNameSurrogate`/`newFileStatFromGetFileInformationByHandle`/
  `sameFile`/`loadFileId`, `file.go` `errPathEscapes`, `io/fs` `ValidPath`,
  and `internal/godebugs/table.go`'s `winsymlink` row. Cited **by symbol**, and
  each row is now classified `contract` or `tripwire`.
- **Claims mechanics**: `C1`…`C92` contiguous, zero duplicates;
  `G1`…`G19` + `G5a` + `G9a` = 21 rows, 8 contract + 13 tripwire = 21.
- **Cross-reference closure**: 129 numbered headings; **every** `§N.N`
  reference in the PRD resolves to a heading that exists (zero unresolved).
- **ADR↔PRD parity**: every `AVP` reference in ADR-034's decision→verification
  table resolves to a declared PRD row; ADR-034 declares D1–D14 with no gaps;
  every decision the review required is present (decision/context, logical
  confinement, platform allowlist, observed-symlink refusal and race limits,
  identity limits and Go upgrade tripwires, one reused scratch buffer,
  status+artifacts same boundary, `winsymlink`/native-Windows policy, no
  `rescap` migration, no provenance persistence, consequences, rejected
  alternatives, acceptance dependencies).
- **Markdown hygiene**: PRD 40 fence markers (balanced), 68 tables with zero
  column-count mismatches; ADR 2 fence markers (balanced), 5 tables with zero
  mismatches; zero trailing-whitespace lines in either; every relative link in
  both resolves on disk; both `json` fenced blocks still parse.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Reviewer focus for rev-3

1. **§7.4.2 and ADR-034 D2 are the load-bearing honesty pair.** The question is
   not "is `os.Root` safer" but "does the pair claim exactly logical pathname
   confinement and nothing more". The quoted doc-comment paragraph is the
   evidence; AVP-189 is the mechanical guard, and it deliberately scans the ADR
   and the PRD as well as the shipped strings, because the over-claim rev-2
   shipped was in the *documents* first.
2. **§7.4.1's allowlist should be checked as *text*, not as intent.** The
   `true` tag must be byte-identical to `$GOROOT/src/os/root_openat.go`'s tag
   and the `false` tag its exact negation. If a reviewer thinks the denylist
   form was adequate, the disagreement to raise is whether a future `GOOS`
   should default to "confined".
3. **§7.4.4's promise is the single most important sentence in rev-3**: "an
   object **observed as different** is never read". Any sentence anywhere —
   PRD, ADR, message, skill — that upgrades it back to "a different object" is
   a defect, and the hard-link case (limit 3) is undetectable *by
   construction*, so no implementation change could close it.
4. **§7.1.1's seam constraint 1 is what keeps the seam from being an escape
   hatch.** Exactly one production implementation, AST-asserted, with a
   sensitivity fixture. If a reviewer can construct a plausible second
   production implementation the constraint would not catch, that is a finding.
5. **§7.4.5's cost statement is deliberately unflattering** (a flat 4,194,305
   bytes on every invocation, including aborts). It should be challenged on
   the merits — Q9 is the escape hatch — not softened.
6. **§14.3.1's exclusion is a security-scope statement, not an omission.**
   Cobra's parse diagnostics are third-party bytes this PRD does not modify.
   The claim to test is that the *hostile-slug* case is still fully covered
   (it reaches `slug-unsafe` and is withheld) and that only the
   mistyped-flag-plus-hostile-slug case is excluded.
7. **AVP-202 proves resolution and arithmetic, not meaning.** Its own doc
   comment says so. Semantic citation review is still the reviewer's job, and
   item 20 above lists exactly what rev-3 found and fixed by hand — a reviewer
   who spot-checks a different sample and finds another mismatch has found a
   real defect.
8. **ADR-034 is `Proposed`, not `Accepted`.** The writer cannot accept its own
   ADR. It becomes Accepted only when it and PRD rev-3 both pass review.

## PRD Writer Result — rev-4 + ADR-034 rev-1 (2026-08-13)

### Files changed (exact, complete)

| File | Change |
|---|---|
| `docs/prds/PRD-artifact-validation-and-provenance.md` | edited in place; `Status: Draft — Awaiting Review (rev-4)`; byline `rev-4 at HEAD be33d2a`; `Architecture` header and `Related` bullet now point at ADR-034 **rev-1** and D1–D18; `Revision history` gains a rev-4 row and records rev-3's disposition as NEEDS REVISION. **4,289 lines** (rev-3: 3,830). |
| `docs/adrs/ADR-034-rooted-filesystem-inspection-boundary.md` | edited in place; `Status: Proposed — Awaiting Review (rev-1)`; byline `rev-1 at HEAD be33d2a`; companion pinned to PRD rev-4; a new `Revision history` table records rev-0's disposition. **945 lines** (rev-0: 635). Decisions D1–D18, in numeric order. |
| `docs/handoff/CURRENT.md` | `Cluster state` flip to `AWAITING REVIEW`, Active Task `Status`/`Rev-4 writer base`/`Scope`, and this section |

Nothing else was touched. **No** edit to `docs/adrs/README.md` (deliberate — see
below), `docs/ROADMAP.md`, `docs/supervisor/LOG.md`, `SPEC.md`,
`docs/whitepapers/**`, `docs/prds/PRD-prepare-intent-bundle.md` (still undrafted,
still blocked), any other `docs/adrs/ADR-0*.md`, `internal/**`, `cmd/**`,
`assets/**`, `tests/**`, `.github/**`, `.wave-close-allowlist`, or any guarded
untracked WIP (`WP-004`, `WP-006`, `WP-007`, `PRD-recurring-patches.md`, the
state-of-the-art case studies).

**Why `docs/adrs/README.md` is untouched.** The dispatch brief scoped rev-4 to
"PRD, ADR-034 and CURRENT" and stated the ADR index needs no status change,
which is correct: ADR-034's status is still `Proposed — Awaiting Review`. The
index line's parenthetical still reads `(rev-0)` and `rev-3`. That is a stale
*revision* label, not a stale *status*, and correcting it would have exceeded
the declared file set. **Flagged for the supervisor**: the index line should be
refreshed to `(rev-1)` / `rev-4` at acceptance, in the same commit that flips
the status to `Accepted`.

### Counts (mechanically verified, commit-independent)

- **PRD 4,289 lines** (rev-3: 3,830; rev-2: 3,075; rev-1: 2,233; rev-0: 1,478);
  **ADR 945 lines** (rev-0: 635).
- **208 acceptance rows** `AVP-001`…`AVP-208` (rev-3: 202), contiguous, zero
  duplicates, zero retired rows. **6 new rows** (`AVP-203`…`AVP-208`); every
  rev-3 ID kept its number, and the rows whose meaning changed (`AVP-084`,
  `AVP-118`, `AVP-139`, `AVP-189`, `AVP-191`, `AVP-193`, `AVP-194`, `AVP-202`)
  were amended in place per §18.1's no-renumbering rule.
- **25 categories**: A 10, B 20, C 8, D 14, E 6, F 5, G 9, H 4, I 6, J 4, K 6,
  L 3, M 6, N 5, O 12, P 10, Q 1, R 9, S 2, T 12, V 17, W 5, X 6, Y 8,
  **Z 20**. Sum = 208, verified by parsing the section headings.
- **By kind**: `U` 61, `I` 96, `S` 6, `G` 31, `S+G` 5, `U+G` 6, `I+G` 1,
  `S+I` 2. Sum = 208.
- **Guard arithmetic**: Kind contains `G` → 31 + 5 + 6 + 1 = **43 rows**
  (rev-3: 39). Complement 61 + 96 + 6 + 2 = 165; 43 + 165 = 208. Note
  `AVP-193` moved `G` → `S+G` (it gained an AST half), so pure `G` went
  30 → 31 rather than 30 → 32.
- **Slice partition verified programmatically**: S1 83, S2 28, S3 49, S4 38,
  S5 10 = 208; zero unassigned, zero double-assigned.
- **Claims audit**: **95 repository claims** `C1`…`C95` (rev-3: 92) and
  **24 Go-stdlib claims** `G1`…`G22` plus `G5a`/`G9a` (rev-3: 21), split into
  **10 `contract`** and **14 `tripwire`**.
- **ADR-034**: **18 decisions** D1–D18 (rev-0: 14), 18 rejected alternatives
  (rev-0: 10), an 18-row decision→acceptance-row dependency table whose every
  `AVP` reference resolves in the PRD.
- **Ladder sizes**: artifact ladder **27 rows** (1–20, 20a, 20b, **20c**,
  21–24); status ladder **20 rows** (1–16, **16a**, 17–19).
- **Catalogs unchanged and still total**: 13 abort codes, 10 reason codes, 10
  advisory codes, 15 lifecycle lines, 9 state-enum values, 9 status outcomes.

### What rev-4 changed, correction by correction

**(1) Platform allowlist → `unix || windows`; `wasip1` unsupported (§7.4.1,
ADR D5, AVP-191 amended, AVP-208 new)**

- `confine_supported.go` → `//go:build unix || windows`;
  `confine_unsupported.go` → `//go:build !(unix || windows)`. Exact negation.
- The platform table gains a **fifth column** (`Supported by this design?`) and
  a separate `wasip1` row so the stdlib's confined set and this design's
  supported set are visibly different objects.
- **rev-3's "byte-identical to `$GOROOT/src/os/root_openat.go`'s tag"
  justification is withdrawn by name.** It reasoned about the *stdlib's*
  implementation set when the question is *this design's*. Four stated reasons
  for excluding `wasip1`: `openFlags()` has two halves not three; the
  `O_NONBLOCK`/FIFO semantics AVP-107/AVP-200 rest on are not WASI preview-1
  semantics; no runner/fixture/cross-build is proposed (which would reproduce
  R7's unexecuted-platform defect); and **no split implementation is specified
  or authorized** — the target is simply refused.
- The asserted property is now a **proper subset** relation: every `GOOS` in
  our `true` set is matched by the stdlib tag, and `wasip1` is matched by the
  stdlib tag and not by ours. AVP-208 owns it, with a sensitivity fixture that
  fails when `wasip1` is re-added without a `wasip1` `openFlags()` half.
- G4 amended; Q8's default sentence updated; one new §22 rejected-alternative
  row; ADR D5 rewritten with the same four reasons.

**(2) Every universal bounded-runtime claim withdrawn (§2.1, §5.1, §7.4.2,
§7.4.3, §8.3, §14.1, §14.4, §15, §16.2, §19 R18, ADR D16 new, AVP-207 new)**

- Three rev-3 sentences are removed and **named** in the withdrawal so a
  reviewer can grep them: "the command has no unbounded wait anywhere" (§8.3),
  "so nothing hangs" (§15), "no leaf kind can hang it" (§7.4.2).
- The guarantee list is rebuilt everywhere it appears (Summary, §2.1 goal 7,
  §7.4.2, §7.4.4, §14.1) as **four** properties: bounded allocation, bounded
  bytes requested, a bounded operation count, and a **non-wedging open** on
  Unix. `O_NONBLOCK`'s scope is stated as "the open, not the read" in §7.4.3,
  AVP-118, AVP-200, G17 and ADR D6/D10.
- New explicit statement, in five places, that an ordinary `read(2)` (and even
  `Lstat`) on a regular file served by a stalled NFS/SMB mount, a wedged FUSE
  server, a `/proc`-style provider or an unresponsive driver **can block
  indefinitely**, and that v1 has **no timeout, no context and no watchdog**.
- §5.1's `--timeout` justification rewritten: the honest reason is "no provider,
  network or subprocess wait to bound, and no cancellation contract defined",
  not "nothing can time out".
- §14.1 gains a fourth threat-model boundary: **availability is out of scope**;
  the security property is confidentiality and integrity.
- §16.2 item 7 extended to forbid any skill surface from making a timing
  promise ("always terminates", "cannot hang", "safe in a blocking preflight").
- **R18 (High)** added; R15's "field hang" phrasing corrected to "wedged open".
- **Q11 (new)** records the additive alternative and states honestly why it is
  not free: Go's `os` file reads are not context-cancellable and
  `SetReadDeadline` does not apply to ordinary files, so a `--timeout` that
  could not interrupt the read would be the same false affordance as rev-2's
  inert `O_NOFOLLOW`.
- **AVP-207** is the mechanical guard. Its row explicitly specifies the
  **quotation-context** rule (a forbidden phrase inside a sentence carrying a
  withdrawal marker is a quotation, not an assertion) plus a fourth *inverse*
  sensitivity fixture asserting the guard is green on the documents as
  written — without which the guard would be permanently red on its own
  withdrawal sections and would be weakened until it proved nothing. **AVP-189
  was amended in place with the same rule**, which it needed already.

**(3) Injectable `SameFile`; exactly two production adapters (§7.1.1, §7.4.4,
ADR D12, AVP-194 amended, AVP-206 new)**

- `RootOps` gains `SameFile(a, b fs.FileInfo) bool`. The seam is now **three +
  three = six methods**; §7.1.1's "five methods wide" and ADR D12's identical
  sentence are corrected.
- Rationale stated: `os.SameFile` is only meaningful over unexported
  `*os.fileStat` values no test outside `os` can construct, so AVP-084,
  AVP-151, AVP-160 and AVP-196 (b) were unwritable — the same defect rev-3
  fixed for `Lstat`/`OpenFile` and left in place one line later.
- **`osFileOps` is now declared explicitly** and returned from `OpenFile`.
  rev-3's sample returned the bare `*os.File`, which made the production
  `FileOps` a type declared in `os` and left AVP-194's "declared outside a
  `_test.go` file" with nothing in `internal/intent` to point at. §7.1.1's code
  sample, its prose, ADR D12 and AVP-194 now all say **exactly two production
  adapters, one per interface, both declared in `internal/intent`** — one
  production implementation per interface, unchanged in spirit.
- Ladder row 13 and status row 9 now read `!ops.SameFile(pre, post)`; §7.4.4
  step 6 likewise; the §7.4.4 race table and AVP-084 updated. G7/G8/G16 and the
  Windows soundness argument are unchanged (they describe what the production
  body delegates to).
- AVP-206 asserts both halves: the injectable verdict (no `os.fileStat`
  construction) and the AST fact that `os.SameFile` appears at exactly **one**
  production call site. R16 updated.

**(4) `Close` in both ladders (§7.4.4 step 11a, §7.5 row 20c, §9.4.2 row 16a,
ADR D15 new, AVP-203/204/205 new)**

- New capture step **11a** (lettered, not renumbered, so every "step 11"
  citation still resolves): close exactly once, **after** the post-capture
  component walk. The ordering is justified, not asserted — holding the
  descriptor across the walk keeps the object pinned so it cannot be unlinked
  and its identity reclaimed while the ancestors are re-observed.
- **Artifact ladder row 20c** → `unreadable`; **status ladder row 16a** →
  `unreadable`. Both sit after the last descriptor-scoped probe and **before**
  any content classification or parse. First-match-wins is stated explicitly:
  20c never overwrites or suppresses an upstream `unstable`/`unreadable`.
- **No new code is minted, and the PRD says why.** Reason `artifact-unreadable`,
  advisory `analysis-sidecar-unreadable`, abort `status-unreadable` — all
  existing. §10.3 gains a paragraph stating the catalogs stay at 10/11/13 so
  AVP-095, AVP-119, AVP-181, AVP-101, AVP-153 keep their arithmetic, and giving
  the reason (a code earns its place by changing the remediation).
- **Truthfulness fix that falls out of this**: `status-unreadable`'s §9.4.5
  message and §10.5.1 lifecycle line are widened to "could not be read **and
  closed cleanly**", because with rev-3's wording a run that read every byte
  and then failed to close would have printed "could not be read". The widened
  form is true of all six of its rows (3, 7, 8, 12, 15, 16a). §10.5.1's
  truthfulness bullet records the change.
- **Zero leaks**: AVP-205 counts opens against closes over every post-open
  ladder row, every status post-open row and all thirteen aborts, asserts zero
  outstanding descriptors when `Inspect` returns, adds an AST half (the close
  is not inside a success-only branch), and ships two sensitivity fixtures
  (skipped close, double close). **R19 (Medium)** added.
- §7.5's ordering list grows from six to **seven** load-bearing orderings;
  the ladder is **27 rows**; §7.6's `unreadable` definition and §15's recovery
  table gain their close rows; §7.1's `Inspect` doc comment states the close
  obligation.

**(5) Cobra parse-error ownership rewritten (§9.5, §10.1, §14.3.1, §5.4,
ADR D17 new, AVP-193 rewritten, C38 amended, C93/C94 new)**

- The mechanism is now stated from source: the root command sets
  `SilenceUsage: true` **and** `SilenceErrors: true`
  (`internal/cli/cobra.go:56-62`), so **cobra prints neither a usage block nor
  the error**; it returns the parse error from `rootCmd.Execute()`, the
  repository's own printer writes `error: %v`
  (`internal/cli/cobra.go:33-39`), and `exitCodeFor` maps the untyped error to
  `1` (`internal/cli/cobra.go:43-52`).
- **rev-3's "and the usage block cobra prints with them" is removed by name**
  — this binary prints no usage block on error, for any command.
- The excluded bytes are correctly characterised as a **repository-emitted line
  wrapping third-party text**, and the PRD states plainly that carrying them
  through our own `Fprintf` does not make them sanitized: the printer is a
  shared pre-existing surface applying no filtering, and changing it would be a
  cross-command behavior change no single command's PRD may make. Those bytes
  are outside the report schema **and** outside the §14.3.3 byte rules.
- **AVP-193's sensitivity fixture is replaced with a real one.** rev-3's
  fixture — "intercepts and re-renders cobra's error inside `RunE`" — is
  **unconstructible**, because a parse error is raised before `RunE`; a guard
  whose negative fixture cannot exist proves nothing. The row is now `S+G`: an
  **AST half** (no `FlagErrorFunc`, no `SetFlagErrorFunc`, no `SetErr`/`SetOut`,
  no self-formatting `Args` validator, no local `Silence*` assignment;
  sensitivity = adding a `FlagErrorFunc`) plus a **behavior half** over the five
  exit-1 inputs (exit 1, empty stdout, exactly one `error:` line matching no
  §9.5 template and carrying no abort code, no report, `.tpatch/`
  byte-identical).
- **C38's description now includes parse errors** and cites C93. C93 (the
  `Silence*` settings) and C94 (`exitCodeFor`'s default-1 mapping) are new.
  §10.1's exit-1 routing row and §9.5's closing paragraph rewritten.

**(6) `eleven` → `twelve` (§9.5, §10.5.2)**

Both occurrences fixed; each cell already listed 5 + 7 = 12 codes. A new
sentence reconciles the arithmetic explicitly: twelve canonical-slug-known
codes plus `slug-unsafe` = the unchanged **thirteen** of §9.4.4, which is the
number AVP-098, AVP-101, AVP-127, AVP-153 and AVP-181 operate over.

**(7) G18 anchor corrected (§23.2, §7.3 step 2, ADR D4)**

The property list (UTF-8, unrooted, slash-separated, no `.`/`..`/empty element,
no leading/trailing slash) lives in the **`io/fs` package documentation's
`# Path Names` section**, not in `ValidPath`'s doc comment, which states only
"valid for use in a call to Open" and links to that section. G18 now anchors
**both** (section for the property, function for the predicate). The row also
records the **`"."` special case** — `fs.ValidPath(".")` is `true` — and notes
that no name this design composes is `"."`, so nothing changes today, but a
future refactor must not assume `ValidPath` refuses it. §7.3 step 2 and ADR D4
carry the same correction.

**(8) ADR D7 name-surrogate table corrected**

`isReparseTagNameSurrogate()` is a **bit test** —
`FILE_ATTRIBUTE_REPARSE_POINT` set **and** `ReparseTag & 0x20000000` — not the
two-tag list its source comment names. rev-0's table answered "no" for *any
other tag*, which is wrong. The corrected table gives each tag's numeric value,
marks AF_UNIX (`0x80000023`) and DEDUP (`0x80000013`) as **non**-surrogates
(so their `ModeDir`/`GetFileType` bits are **not** suppressed), and states that
"any other tag" **depends on that tag's own bit** — `ModeIrregular` either way,
but the suppression differs. A third consequence is added: the two exceptions
are exceptions to the *surrogate* rule as well as the *mode* rule, which is
precisely why the `!IsRegular()` kind gate — not the refusal predicate — must
be total. `winsymlink=1` is retained unchanged. The PRD's §7.4.3 table gets the
same numeric values and the same "depends on the bit" wording.

**(9) Exit-3 workspace divergence disclosed (§9.2, ADR D18 new, C95 new)**

`tpatch status` outside a workspace exits **1** (untyped
`FindProjectRoot` error via `openStoreFromCmd` → `exitCodeFor`'s default);
`tpatch prepare <slug> --check` exits **3** with a full abort report. §9.2 now
states the divergence, justifies it on three grounds (per-command exit
contracts per `SPEC.md:135-141`; this command's exit code is a *verdict* and
collapsing to 1 would make it the only nonzero path with no report, breaking
the `artifacts` ⇔ `abort` invariant; additive and reversible — no existing
command changes and this command never calls `openStoreFromCmd`), and states
what is **not** claimed: a harness grepping exit 1 for "no workspace" will not
get that answer here, and §16.1's `SPEC.md` row must document the envelope.

**(10) Fixed-buffer rationale corrected (§7.4.5, ADR D9, §22, G20/G21/G22 new)**

- **`io.ReadAll(io.LimitReader(f, Max+1))` is bounded.** The limit reader caps
  the result, so total allocation is `O(Max)`. rev-1 claimed a false *exact
  ceiling*; rev-2/rev-3 over-corrected into "the allocation is not bounded",
  which is equally false. rev-4 states both accurately and forbids the
  unboundedness claim.
- The real rejection is cost **shape**: `ReadAll` grows by `append`, so one
  capture performs a *sequence* of increasing allocations with copies, and the
  sequence is paid again on each of **five** sequential captures. The chosen
  reusable fixed buffer **caps and flattens** that cost.
- **The ~4 MiB zeroing cost is now stated**: `make([]byte, n)` yields a zeroed
  slice (G22), so the invocation pays one zeroing pass — once per invocation,
  never per capture. Q9's note updated (lazy allocation removes the cost only
  for runs that open nothing).
- **Cap↔message coupling is restated exactly and unchanged**: AVP-201 *derives*
  the unit string from the constant (never compares two literals), asserts
  appearance in exactly the enumerated messages and non-appearance of any other
  limit figure, and fails in **both** directions.
- G20 (LimitReader bounds → contract), G21 (ReadAll growth shape → tripwire)
  and G22 (zeroed `make` → contract) added; §22's `ReadAll` row rewritten;
  R17 updated.

**(11) Walk→`Lstat` race limit retained and sharpened (§7.4.4, §8.3, ADR D8)**

The residue is restated in the place most likely to be read as stronger than it
is: an object *observed as different* is never read, **but** an object never
observed to be different — most importantly a consistent in-root alias or hard
link that satisfies every probe — **is** read, and its bytes are then
**attributed to the canonical artifact name** in the report. The claim is "the
object I classified is the object I read", not "the canonical name designated
this object at every instant". Limits 1, 2 and 5 in §8.3, the §7.4.4 limit
list, ADR D8 and R5 all carry the attribution sentence.

**(12) Cap-message coupling and Windows junction mechanism unchanged**

`cmd /c mklink /J`, `t.Fatal` never `t.Skip`, the source half forbidding
`t.Skip`/`t.Skipf`/`t.SkipNow` in the fixture path, and `runtime.GOOS !=
"windows"` as the only permitted guard — all verbatim from rev-3 (§16.1,
AVP-199, ADR D13). AVP-201's mechanism likewise, now stated explicitly as
derive-not-compare and fail-in-both-directions.

**(13) IDs, counts, categories, kinds, slices, claims, citations, guards**

All updated and mechanically re-verified — see "Validation performed".

### Unchanged from rev-3 (deliberately, as instructed)

- **`os.Root` as the mechanism** and the whole D1/D3/D4 policy.
- **Logical (not physical) confinement**, §7.4.2's table, the quoted `Root`
  doc-comment paragraph, AVP-189, AVP-190, R14, ADR D2.
- **Full three-doc readiness** (§6.2): `ready` requires `analysis.md` +
  `spec.md` + `exploration.md`; the sidecar stays optional.
- **`status.json` safety and `FeatureState` validation**: same boundary, same
  caps, all nine populations, thirteen abort codes, fifteen lifecycle
  annotations, valid-state gate before any echo.
- **`provenance` is the stable constant `unknown`**; no ADR trigger fired
  (ADR D14).
- **Manual/routing compatibility**: loose `--manual`, `next` and `cycle`
  unchanged; AVP-064…AVP-069 and AVP-130…AVP-133 still pin the loose behavior.
- **No mutation, no new state**; **the `prepare` reserved-surface block**
  (exit 4) and the unregistered `--manual`/`--regenerate`.
- **`PRD-prepare-intent-bundle.md` remains blocked and undrafted** (§20).
- **ADR status**: `Proposed — Awaiting Review`. The writer cannot accept its
  own ADR.

### Open decisions left to review (§21, eleven)

Q1 `--all`; Q2 exit `4` as a cross-command convention; Q3
`MaxArtifactBytes = 4 MiB`; Q4 `request.md` as a fifth row; Q5
platform-conditional Windows reserved-device refusal; Q6 `--format` aliasing;
Q7 `MaxStatusBytes = 1 MiB` and its separateness; Q8 compile-time vs runtime
platform refusal; Q9 lazy vs unconditional scratch allocation; Q10 whether the
seam should be unexported; **Q11 (new)** whether v1 should bound wall-clock
with a `--timeout` or a cancellable context. All eleven have a stated default;
none blocks review.

### Implementation status

**None.** Planning only. No Go file, test, asset, workflow or CLI surface was
created or modified. `cmd/tpatch/main.go`, `.github/workflows/ci.yml` and
`SPEC.md` remain named as **required future** edits (§16.1) and were **not**
touched.

### Validation performed (docs-only change)

- **AVP mechanics**: 208 declared rows, contiguous `AVP-001`…`AVP-208`, zero
  duplicates; **every** `AVP-NNN` token in the PRD's prose **and in ADR-034**
  resolves to a declared row (zero unresolved); category counts parsed from the
  section headings sum to 208 and match §18.27; kind counts parsed from the
  Kind column sum to 208 and match; the guard predicate yields **43**; §17's
  slice assignment expanded programmatically and verified as a partition of
  1…208 (S1 83, S2 28, S3 49, S4 38, S5 10 — zero duplicates, zero missing,
  each slice's stated count equal to its expanded count).
- **Guard-count self-check caught a writer error**: the first draft of §18.27
  stated pure `G` = 32 and guard = 44. The parser returned 31 and 43 because
  `AVP-193` moved out of pure `G` into `S+G`. §18.27, §18.28 and AVP-139 were
  corrected to 31/43 and the narrative now explains the move.
- **Claims mechanics**: `C1`…`C95` contiguous, zero duplicates;
  `G1`…`G22` + `G5a` + `G9a` = 24 rows, 10 contract + 14 tripwire = 24.
- **Repository anchors**: every `file:line` citation in **both** documents
  parsed and checked in-range; zero out-of-range, zero missing files. The four
  new/changed ones were content-verified by reading the cited lines —
  `internal/cli/cobra.go:56-62` (`SilenceUsage`/`SilenceErrors` on the root
  command struct, C93), `:43-52` (`exitCodeFor`'s default `return 1`, C94),
  `:33-39` (the `Fprintf` printer, C38), `:3782-3793` +
  `internal/store/store.go:23-40` (`openStoreFromCmd` → `FindProjectRoot`
  returning a plain `errors.New`, C95).
- **Go stdlib claims re-read from the pinned toolchain's `GOROOT`**
  (`go1.26.5`): `os/types_windows.go` `(*fileStat).mode`,
  `isReparseTagNameSurrogate` (confirming the `0x20000000` bit test and the
  AF_UNIX/DEDUP branches verbatim, including the DEDUP explanatory comment),
  `Mode`/`modePreGo1_23`; `io/fs/fs.go` package doc `# Path Names` section and
  `ValidPath`'s body (confirming the `"."` special case returns `true`);
  `os/root_openat.go`'s build tag; `os/root_unix.go` `rootOpenFileNolog`.
- **Cross-reference closure**: every `§N.N` reference in the PRD **and** in
  ADR-034 resolves to a PRD heading that exists (zero unresolved).
- **ADR↔PRD parity**: ADR declares D1–D18 with no gaps, in numeric order (D13
  and D14 were moved back above D15 after the new decisions were appended);
  every `AVP` reference in the ADR's 18-row decision→verification table
  resolves to a declared PRD row; every ADR decision D1–D18 is cited by name
  somewhere in the PRD; the ADR's companion pointer, references block and
  consequences all name rev-4.
- **Markdown hygiene**: PRD 44 fence markers (balanced), 69 tables with **zero**
  column-count mismatches (counted with `\|` escapes stripped, which is what
  the earlier "mismatch" noise was); ADR 2 fence markers (balanced), 6 tables
  with zero mismatches; **zero** trailing-whitespace lines in either; every
  relative link in both resolves on disk; both `json` fenced blocks parse.
- No Go source changed, so `gofmt` / `go build` / `go test` are not applicable
  to this change set; no existing docs test targets these files
  (`internal/workflow/docs_totality_guard_test.go` reads exactly three
  verify-family documents, none of them touched here).

### Reviewer focus for rev-4

1. **The `wasip1` exclusion is a *narrowing*, and it costs a target.** rev-3
   would have run confined on `wasip1`; rev-4 refuses it. The argument is that
   confinement is necessary but not sufficient — the design also needs an
   `openFlags()` half and a runner, and has neither. If the reviewer thinks
   `wasip1` support is worth a third build-tagged half, that is a scope
   *addition* with its own slice, not a build-tag edit; **rev-4 deliberately
   does not split the implementation**.
2. **The bounded-runtime withdrawal is the largest semantic change.** Nothing
   about the *code* changes — the same `O_NONBLOCK`, the same kind gates — only
   what the documents promise. Check §7.4.2, §8.3 and §14.1 for any residual
   assertion, and check that AVP-207's quotation-context rule is specified well
   enough to be implementable: the guard must tolerate the withdrawal
   sections' own quotations of the removed sentences, which is why it ships a
   fourth, *inverse* fixture asserting green-on-current-text.
3. **`Close` mints no new code, deliberately.** The alternative (a fourteenth
   abort code) is argued down in §22 and ADR D15 item 5. The visible cost is
   that `status-unreadable`'s frozen message and lifecycle line changed text;
   if the reviewer prefers a new code, that reopens the thirteen-code
   arithmetic in five places (AVP-095, AVP-098, AVP-101, AVP-153, AVP-181).
4. **`SameFile` is one production method that exists only for testability.**
   Its production body is a one-line delegation. If the reviewer objects to
   widening the interface for tests, the counter-argument is in §7.1.1 and
   ADR D12 constraint 3: without it, four identity rows are unwritable, which
   is the exact defect class rev-3 was revised to fix.
5. **Two production adapters, one per interface** — this is a *clarification*
   of rev-3's "exactly one production implementation", not a relaxation. rev-3
   said one implementation of *each* interface but its code sample returned a
   bare `*os.File`, which put the `FileOps` implementation in package `os`.
   Check §7.1.1 rule 1, ADR D12 constraint 1 and AVP-194 agree on this.
6. **Ladder row lettering.** 20c and 16a are lettered so no rev-2/rev-3 row
   citation breaks. Verify the ordering claims: 20c after 20a/20b, before
   21–24; 16a after 16, before 17.
7. **AVP-193 is the one row whose *kind* changed** (`G` → `S+G`), which is why
   the guard arithmetic moved 39 → 43 rather than 39 → 45. §18.27 explains it.
8. **`docs/adrs/README.md` is untouched by design** and its parenthetical
   revision labels are now stale. That is flagged above for the supervisor to
   fix at acceptance, not silently left.

## WP-005 Turn 2 Scope

- Append one `agreement` turn introducing the new stable agent ID.
- Record the council method and the advisor/reviewer split honestly.
- Add `## Agreed` findings: validation before orchestration, `prepare --check`
  first, no new lifecycle state, optional internal SDD, downstream SDD
  encouraged but never enforced.
- Resolve WP-005's prior "separate whitepaper" uncertainty: this paper is
  sufficient prior art for two gated PRDs.
- Keep the paper `Exploring` until the first graduating PRD is drafted.
- Do not edit code, create an ADR, or add prepare to the implementation
  roadmap in this task.

## WP-005 Turn 2 Writer Result

- Added CO76 as the second stable whitepaper author and appended one
  `agreement` turn without editing Turn 1.
- Recorded the full council split (2 ship now / 2 reject / 1 specify first),
  five-reviewer convergence and chairman disposition.
- Added an unbylined `## Agreed` section covering validation before
  orchestration, check-first delivery, no new state, optional internal SDD and
  downstream encourage-never-enforce.
- Replaced the open-ended "separate whitepaper" product note with two ordered
  PRDs: artifact validation/provenance first; prepare bundle blocked second.
- Kept `Status: Exploring` until the first graduating PRD is drafted.
- Added current-code anchors for manual validation, Path A's structured
  analysis sidecar and `next`'s file-presence inference.

## Turn 2 Review Adjudication

- **Internal**: NEEDS REVISION — WP-004 is guarded/untracked, so WP-005's
  Related link breaks in a clean checkout.
- **External**: APPROVED WITH NOTES; five valid findings are folded:
  1. provenance must be `unknown` until a representation is chosen, and the
     prerequisite PRD may trigger an ADR at decision time;
  2. all-or-nothing publication includes intent artifacts, sidecars and
     `status.json`, not just staged Markdown;
  3. compare `prepare` explicitly with `cycle`, `next` and the individual
     manual phases;
  4. richer validation is advisory for existing routing unless a later PRD
     enumerates a deliberate behavior change;
  5. map each council lens to its position in the durable record.
- **Coupled observation accepted**: update the whitepaper index for all tracked
  WP-002/WP-003/WP-005 documents.
- **Protocol**: Turn 2 remains immutable; corrections append as Turn 3.

## Turn 3 Revision Result

- Removed the broken guarded-WP-004 link without staging unrelated WIP.
- Mapped every advisor lens to its position and explained the 3/2 peer-review
  selection that produced the validation-first verdict.
- Defined legacy/current provenance as `unknown` until accepted persistent
  metadata proves otherwise; PRD-1 may trigger an ADR when it makes that
  choice.
- Expanded atomic publication to Markdown, structured sidecars and
  `status.json`; distinguished shared validation from incremental phase
  writers.
- Added the required existing-primitives pre-flight for individual
  `--manual`, `cycle` and `next`.
- Made slice-1 validation advisory and routing-compatible by default.
- Refreshed the tracked whitepaper index for WP-002, WP-003 and WP-005.

## Turn 3 Review Verdict

- **Internal**: APPROVED.
- **External**: APPROVED WITH NOTES.
- Every rev-0 finding is closed.
- LOW N1 closed by attributing council versus review-derived Agreed items.
- LOW N2 is pinned as a required first-PRD acceptance decision: report-only
  versus stronger mutating `--manual` gates and their compatibility.
- Turn 4 records the disposition append-only.

## Post-Release Review Adjudication

- **External verdict**: APPROVED WITH NOTES.
- **F1 LOW — valid**: Wave C extraction left 16 accepted-contract
  `verify.go:<line>` anchors stale; re-resolve them against current symbols
  and refresh the validation marker.
- **F2 LOW — valid**: `TestAcceptanceLedger_TestsExist` can false-pass on a
  comment because it searches raw bytes. Replace it with package-aware AST
  resolution and a sensitivity regression.
- **Observation — deferred explicitly**: the wave-close untracked-source
  sentinel cannot enumerate source files inside an untracked nested Git
  repository. Registered as GH #9; no release invariant is invalidated.
- **Tag policy**: fold only on `main`; never move v0.15.1.
- **Next planning work, deferred one turn by the operator**: append WP-005
  council Turn 2, then draft artifact-validation/provenance and prepare-bundle
  PRDs through the normal writer/review loop.

## Post-Release Fold Result

- Every verify-family citation in ADR-013 and PRD-verify-freshness was
  re-resolved against the extracted source at tag `15560af`; a non-vacuous
  bounds guard now checks the full reference set.
- Stale present-tense `active`-parent prose was corrected to describe the
  shipped Wave C behavior and its current source sites.
- The 161-row ledger now resolves an exact package, runnable
  `func TestX(*testing.T)` declaration, and optional literal subtest through
  Go AST. Comments, wrong/external packages, invalid signatures, unused table
  names and unrelated literals all fail sensitivity fixtures.
- Three internal review passes closed package/signature/subtest and citation
  scope/semantics findings. The final WIP-file observation was dismissed:
  those files predate this task, are guarded, and have an empty diff.
- Full uncached suite, vet and build pass.
- GH #9 records the deferred nested-repository source-sentinel blind spot.
- v0.15.1 remains byte-fixed at `15560af`; the fold is main-only at `64010bf`.

## Rev-3 Review Verdict

- **Internal**: APPROVED.
- **External/original reproducer**: APPROVED.
- No high-confidence defect remains.
- The original pre-land → land → post-land sequence and `verify --all` pass.
- The C-locale, offline apply-classifier grammar resisted fatal, mixed,
  wrapper, signalled, translated and broad-phrase adversarial outcomes.
- All rev-0 through rev-2 findings remain closed.

## Session Summary — rev-3 fold

**P1 — deterministic locale.** `internal/gitutil/trailers.go` appends
`GIT_NO_LAZY_FETCH=1` and the new `CLocaleEnv` (`LC_ALL=C`) LAST to
every evidence invocation, so both win over the inherited environment
AND any caller-supplied extra. That covers `log`, `rev-parse`,
`read-tree`, `apply`, `diff`, `cat-file` and `merge-base` — rev-2
forced the locale only on the `-C0` ladder step, leaving every other
classified diagnostic at the mercy of the ambient locale. The
`ForceCLocale` option is removed rather than left as a second, weaker
path. The workflow gateway's `offlineEnv()` carries both entries too, so
the shadow worktree commands are equally deterministic. Unrelated
callers are untouched: `shadowEnv(nil)` still returns nil.

**P1 — narrow, safe `ApplyProbeAnswered`.** The broad substring list
(`already exists`, `new file`, `deleted file`, `patch does not apply`,
…) is gone. The rule is now:

- success → answer;
- exit 1 → answer, by exit code alone (measured: every ordinary
  conflict exits 1, so no text is consulted);
- any exit other than 0/1/128 → FAILURE regardless of stderr, which
  covers signalled and unstartable processes (-1), 2, 126, 127, 129+;
- exit 128 with a missing-object or network diagnostic → FAILURE;
- exit 128 → answer ONLY when every non-empty stderr line matches the
  anchored C-locale malformed-patch grammar, with at least one
  recognised diagnostic line.

The grammar is `IsMalformedPatchDiagnostic`: anchored regexps for
`No valid patches in input (allow with "--allow-empty")`, `corrupt patch
at <path>:<line>` and `at line N`, `patch fragment without header`,
`patch with only garbage at line N`, and `corrupt binary patch`
(including its paired no-valid-patches line), plus `Checking patch …`
as an informational line that can never satisfy the requirement alone.
A `fatal:` spoof, a wrapper line, an unknown line, an empty diagnostic
or any mixture is rejected.

Measured on git 2.55.0 under `LC_ALL=C` (probe repo removed):

| input | exit | stderr |
|---|---|---|
| empty / garbage / prose | 128 | `error: No valid patches in input (allow with "--allow-empty")` |
| truncated or garbage hunk | 128 | `error: corrupt patch at ../p.patch:5` |
| fragment without header | 128 | `error: patch fragment without header at ../p.patch:1: @@ …` |
| corrupt binary payload | 128 | `error: corrupt binary patch at …` + the no-valid-patches line |
| new file that already exists | **1** | `error: f.txt: already exists in index` |
| delete of a missing file | **1** | `error: gone.txt: does not exist in index` |
| context mismatch | **1** | `error: f.txt: patch does not apply` |

`unrecognized input` was NOT reproducible on this git and is therefore
excluded — the classifier fails closed to `unavailable` for anything it
cannot demonstrate.

## Current State

- GH #8 stays empirically closed with the rev-3 binary, including under
  a foreign ambient locale (`LC_ALL=fr_FR.UTF-8`): before land passes,
  after land `landing evidence: exact` with a dual-anchor baseline, and
  `verify --all` exits 0.
- AC-L68 / AC-L69 remain green against a real filtered remote.
- No blocker.

## Files Changed (rev-3)

New:

- `internal/gitutil/apply_classifier_test.go`
- `internal/workflow/verify_rev3_fold_test.go`

Modified:

- `internal/gitutil/trailers.go` (`CLocaleEnv`, unconditional locale,
  anchored grammar, exit-code-led `ApplyProbeAnswered`; `ForceCLocale`
  removed)
- `internal/workflow/verify_gitgate.go` (`offlineEnv` carries the
  locale)
- `internal/workflow/verify_landed.go` (ladder no longer passes a
  per-call locale flag)
- `internal/workflow/acceptance_ledger_test.go`
- `CHANGELOG.md`, `docs/handoff/CURRENT.md`

## Test Results

- `gofmt -l .` clean; `go vet ./...` clean; `GOOS=linux` and
  `GOOS=windows` vet clean.
- Cross-builds clean: linux/amd64, linux/arm64, darwin/arm64,
  windows/amd64.
- `go test -count=1 ./...` — all 12 packages pass.
- `go test -race -count=1 ./internal/workflow ./internal/gitutil
  ./internal/store ./internal/cli` — pass.
- **Regression proof.** Against the rev-2 production files, 6 rev-3
  workflow sub-tests fail (four broad-phrase historical-V8 exits, the
  ladder broad-phrase case, and the C-locale assertion). The classifier
  itself is proven in-tree by
  `TestRev3ClassifierFixesRev2Misclassifications`, which reproduces the
  rev-2 predicate verbatim and enumerates ten outcomes it promoted to a
  patch verdict and rev-3 rejects;
  `TestRev3ClassifierKeepsEveryRealAnswer` proves no real git answer was
  lost.
- Real-git goldens: `TestRealGit_MalformedPatchesExit128AndAreAdmitted`
  and `TestRealGit_OrdinaryConflictsExitOne` measure the actual
  diagnostics rather than mocking them.
- Acceptance ledger: 161/161 rows mapped; AC-L26/L58/L69/L117/L123/
  L129/L134 remapped to the rev-3 black-box and golden proofs.
- Docs totality guard (AC-L135 / G1–G10): zero hits; accepted documents
  untouched.
- GH #2 regression green and unmodified (AC-L121 diffs it against
  WAVE_BASE `b768602`).
- Side Research md5: `b385fe622db9926f48861105239f113e`.

## Next Steps

1. Record a fresh `origin/main` WAVE_BASE.
2. Dispatch `PRD-prepare-intent-bundle.md` through the writer/review loop.
3. Keep all implementation blocked until that PRD is accepted.

## Blockers

None. Planning is accepted; implementation remains a separate future wave. Two items are **gated on review rather than blocked**:

1. **ADR-034 is `Proposed`, not `Accepted`.** A writer cannot accept its own
   ADR. It and PRD rev-3 are reviewed together; acceptance of both is the
   precondition for dispatching implementation slice S1.
2. **`PRD-prepare-intent-bundle.md` remains blocked** and undrafted until this
   PRD is accepted (§20).

No implementation is authorized. `cmd/tpatch/main.go` and
`.github/workflows/ci.yml` are named as required *future* edits and were not
touched.

## Rev-2 Review Adjudication

- **Internal**: NEEDS REVISION (4 HIGH, 2 MEDIUM).
- **External**: NEEDS REVISION; all rev-1 findings closed, then bounded
  `os.Root`, seam, citation and platform-policy findings.
- **Supervisor verdict**: NEEDS REVISION → rev-3 + ADR-034.
- **Disposition**: every item below is **closed in rev-3** — see "PRD Writer
  Result — rev-3 + ADR-034 rev-0" for the point-by-point mapping.

### Architecture decision now requiring ADR

Adopting a held `*os.Root` as a new read-only rooted namespace is a non-obvious
repository architecture choice. It differs from the shipped
`rescap.GatePath` pathname model and sets platform/confinement/identity policy.
Rev-3 must create proposed
`ADR-034-rooted-filesystem-inspection-boundary.md` and review it with the PRD.
This is **not** the deferred provenance ADR; provenance remains constant
`unknown`.

### Rev-3 required corrections

1. Scope `os.Root` honestly: logical root confinement, not physical filesystem
   boundary confinement; bind mounts/filesystem boundaries remain reachable.
2. Use allowlist build tags (`unix || windows || wasip1`) and fail closed on
   other targets; validate slug before platform selection.
3. Remove `EnsureSafeRepoPath` misuse and use canonical `fs.ValidPath`
   root-relative names.
4. Correct Windows reparse mapping, pin `winsymlink=1`, and make native Windows
   junction tests fail rather than skip.
5. Define injectable rooted/file operation seams for deterministic race/error
   rows without weakening production call-graph guards.
6. Reuse one fixed cap-plus-one data buffer per inspection and document cost.
7. Weaken identity claims to “objects observed as different are never read”;
   document inode/file-ID reuse and same-identity aliases.
8. Pre/post-walk components per capture; describe the residual swap-back
   limitation.
9. Scope attacker-byte guarantees to command-owned output; Cobra parse errors
   remain generic CLI behavior.
10. Correct every drifted AVP citation and add a citation-resolution guard.
11. Couple cap values to frozen messages, define Windows test mechanism, and
    close the remaining status/output wording.

## Rev-3 Review Adjudication

- **Internal**: NEEDS REVISION (4 HIGH, 1 MEDIUM plus one LOW).
- **External**: APPROVED WITH NOTES; architecture accepted, four contract
  parity corrections required before S1.
- **Supervisor verdict**: NEEDS REVISION → narrow rev-4.

### Rev-4 required corrections

1. Remove `wasip1` from the supported allowlist (or define a separate
   compilable flag contract); this revision chooses fail-closed
   `unix || windows`.
2. Withdraw universal no-unbounded-wait claims. Allocation is bounded; ordinary
   OS reads can still block. Static/raced FIFO handling remains tested.
3. Add an injectable `SameFile` operation to the test seam so identity rows are
   implementable without constructing private `os.fileStat` values.
4. Add `Close` failures to status/artifact ladders and AVP coverage.
5. Correct Cobra parse-error ownership: the shared root printer emits pflag
   text unsanitized; those generic exit-1 bytes are outside this command's
   report schema.
6. Align `RootOps`/`FileOps` samples, ADR D12 and AVP-194 on two adapters.
7. Fix the twelve-code arithmetic, G18 `fs.ValidPath` anchor, Windows
   name-surrogate wording and workspace-exit divergence disclosure.
8. Preserve ADR decisions, update matrix/claims/counts, and re-review only the
   affected contract surfaces.

## Rev-1 Review Adjudication

- **Internal**: NEEDS REVISION (2 HIGH, 3 MEDIUM).
- **External**: NEEDS REVISION; every rev-0 finding closed, then three
  blocking status/output findings plus bounded completeness notes.
- **Supervisor verdict**: NEEDS REVISION → rev-2.
- **Disposition**: every item below is **closed in rev-2** — see "PRD Writer
  Result — rev-2" for the point-by-point mapping.

### Rev-2 architecture correction

Go 1.26's `os.Root` is the cross-platform rooted namespace primitive. Rev-2
must open one root for the repository and use root-relative `Lstat`/open for
`status.json` and all artifacts. This closes ancestor escape without custom
pathname re-resolution:

1. Refuse observed symlink components.
2. Before reading, compare rooted `Lstat` identity to the opened handle's
   `File.Stat` identity and recheck kind/size.
3. Use final no-follow/nonblocking flags on Unix. On Windows rely on
   `os.Root`'s handle-relative implementation and handle-derived stat identity,
   not ordinary `os.Lstat`/`os.SameFile`.
4. Read into one preallocated `MaxArtifactBytes+1` buffer/loop; do not claim
   `io.ReadAll(LimitReader)` has an exact allocation ceiling.

### Remaining corrections

- Apply the same safe bounded capture to `status.json`.
- Validate `FeatureState`; never echo unknown lifecycle bytes.
- Define human lifecycle text for status present/absent/unreadable/malformed/
  invalid-state populations.
- Close abort messages and correct `--path`/workspace error ownership.
- Correct quiet-abort guidance, control-byte wording, guard arithmetic,
  canonical hand-assembled feature scope and skill wording for ordinary exit 2.
- Rebuild AVP totals around all new status/root/race cases.

## Context for Next Agent

Reviewer focus for rev-3:

1. **Exit 1 is decided WITHOUT reading stderr.** That is what makes the
   grammar safe to narrow: every ordinary conflict — including the
   `already exists` / `does not exist` / `does not apply` forms rev-2
   listed — exits 1, so removing them from the grammar cannot cost a
   real answer. `TestRealGit_OrdinaryConflictsExitOne` measures it.
2. **Exit 128 is the only text-sensitive case**, and it is required:
   git reports a malformed or empty patch with exit 128, and two shipped
   GH #2-era rows depend on that staying a patch-level answer.
3. **The grammar fails closed.** A form that could not be demonstrated
   on this git (`unrecognized input`) is excluded; an unrecognised
   diagnostic yields `unavailable` (R10), which is the honest "the
   reader could not answer" outcome rather than a fabricated patch
   verdict.
4. **Locale is applied at the ONE env builder**, so there is no second
   path that can drift. `ForceCLocale` was deleted rather than kept as a
   no-op, to remove the ambiguity of two mechanisms for one guarantee.

Carried forward from rev-0/rev-1/rev-2 (still true):

- `landing_evidence.state` is OMITTED for the D10 artifact-presence
  short-circuit; `failed_at` and `reason` carry the outcome.
- Classification terminals run before the static short-circuit and
  allocate no shadow; unanswerable probes route through
  `terminalEvidencePhase` so remediations stay inside the closed
  R10/R22 set and `failed_at` inside the thirteen-value vocabulary.
- V2 parses the captured recipe bytes; the verify path has no live
  artifact read (AST-guarded).
- AC-L84 and AC-L111 are proven in two halves because V4 pre-empts the
  arbitration branch; AC-LD18a's refusal half is driven through the
  production validator.
- The isolated index lives under `<git-dir>/tpatch-verify`.
- Stray build artifacts in the repo root predate the session and are
  gitignored; they were left untouched.

## Side Research — State-of-the-art middle pass (2026-05-10)

Paper-only exploratory pass completed for a non-LLM middle layer between
deterministic reconcile heuristics and full provider/coding-agent workflows.
This does **not** change code, schema, CLI behavior, roadmap status, PRDs, or
ADRs.

### Research packet

Created `docs/state-of-the-art/` with docs modeled after the existing market
research / PRD conventions: header block, related links, refresh triggers,
references, open questions, and disputes.

Files:

- `docs/state-of-the-art/README.md`
- `docs/state-of-the-art/patch-theory-and-commutation.md`
- `docs/state-of-the-art/patch-identity-and-structural-fingerprints.md`
- `docs/state-of-the-art/search-based-patch-application.md`
- `docs/state-of-the-art/experiment-guide-structural-middle-pass.md`
- `docs/state-of-the-art/tpatch-metadata-for-patch-identity.md`
- `docs/state-of-the-art/patch-capture-context-research-brief.md`
- `docs/state-of-the-art/patch-capture-prior-art-and-hooks.md`
- `docs/state-of-the-art/research-roadmap.md`
- `docs/state-of-the-art/tpatch-middle-pass-synthesis.md`

### Findings

1. Patch theory is useful as vocabulary for identity, inverse, composition,
   commutation, dependency, and conflict, but tpatch should not claim
   Darcs/Pijul guarantees on top of unified diffs.
2. Patch identity should be treated as a ladder: exact bytes, `git patch-id`,
   token fingerprints, AST/CFG/PDG similarity, behavioral checks, and finally
   provider/human intent judgment.
3. Computer-vision feature matching maps to code relocation: detect salient
   code keypoints, compute local descriptors, match across old/new upstream,
   reject outliers, then attempt relocated apply in a shadow tree.
4. Search-based application should operate only on uncertain patch clusters,
   after deterministic dependency/commutation pre-passes shrink the search
   space.
5. Beam search is the likely first practical non-LLM planner; MCTS and
   evolutionary algorithms remain candidates for larger uncertain clusters.
6. Vector retrieval / RAG fits as a distinct middle layer: dense retrieval can
   rank likely patch/hunk/code-region matches below full provider reasoning,
   while generation over retrieved context still belongs to the provider tier.
7. The experiment guide defines collection formats for feature metadata, hunks,
   keypoints, fingerprints, retrieval results, commutation relations,
   candidate apply attempts, metrics, and ground-truth labels.
8. First-party tpatch metadata should be the happy path for tpatch-aware repos:
   current metadata is good for lifecycle/DAG reasoning, but future patch
   generations, dependency version snapshots, operation IDs/read-write sets,
   structural anchors, relation artifacts, and vector manifests would make
   identity and ordering easier before fuzzy fallback.
9. A new patch-capture research brief preserves this PRD/ADR queue and defines
   the next front: Quilt-style explicit file claims, Git index/hook boundaries,
   IDE hooks, coding-agent event logs, and privacy-safe agent context capture.
10. Entire is verified as a concrete prior-art target. Its model uses Git hooks,
    agent hooks, commit trailers, a separate `entire/checkpoints/v1` metadata
    branch, shadow checkpoints, full transcript/session storage, redaction, and
    optional checkpoint remotes. tpatch should borrow the Git-native linking
    pattern but default toward summaries/references over raw transcripts.
11. `docs/state-of-the-art/research-roadmap.md` is now the durable exploratory
    tracker so research can advance independently if `docs/handoff/CURRENT.md`
    is reassigned to implementation work.
12. Amendment models differ by tool: Quilt/StGit usually refresh the managed
    patch, Git supports both amend and fixup/squash-forward workflows, Aider
    favors small commits plus undo, and Entire preserves context links around
    rewrites. tpatch likely needs canonical-current patch plus append-only
    generations, with explicit amend/fixup/fold/fork semantics.

### PRD drafts promoted from research (2026-05-13)

The first capture/metadata foundation PRDs were drafted as paper-only planning
docs:

- `docs/prds/PRD-feature-file-claims.md`
- `docs/prds/PRD-record-capture-modes.md`
- `docs/prds/PRD-feature-patch-identity-metadata.md`
- `docs/prds/PRD-feature-patch-amend.md`

`docs/state-of-the-art/research-roadmap.md` is updated to point at these drafts.
The remaining gate before implementation is review/acceptance of the queued
capture privacy and amendment-policy ADRs plus PRD review.

### Candidate follow-up names

These are research outputs only, not queued roadmap work. Four items below now
have draft PRDs as noted above.

- `PRD-structural-patch-fingerprints`
- `PRD-feature-patch-identity-metadata`
- `PRD-dependency-version-snapshots`
- `PRD-recipe-operation-identity`
- `PRD-structural-anchor-manifest`
- `PRD-patch-vector-index`
- `PRD-reconcile-commutation-graph`
- `PRD-reconcile-search-planner`
- `ADR-structural-middle-pass-boundary`
- `PRD-reconcile-planner-audit-artifacts`
- `PRD-feature-file-claims`
- `PRD-record-capture-modes`
- `ADR-patch-amendment-policy`
- `PRD-feature-patch-amend`
- `PRD-active-feature-session`
- `PRD-agent-event-log`
- `PRD-ide-capture-hooks`
- `PRD-git-hook-capture-guards`
- `ADR-capture-context-privacy-boundary`
- `ADR-capture-metadata-branch`
- `PRD-record-context-summary`
