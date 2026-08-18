# copilot-api Cumulative Verification and Migration Feedback

**Status**: Verified backlog evidence — 2026-08-18; no implementation authorized
**Date**: 2026-08-18
**Owner**: Core
**Evidence revision**:
[`tesseracode/copilot-api@e2d7ce4`](https://github.com/tesseracode/copilot-api/commit/e2d7ce457f11ba077f508c360adac03a4db0e8ad)
(`.tpatch/RETROSPECTIVE.md`, Part 5)
**Issues**:
[GH #18 cumulative verify](https://github.com/tesseracode/tesserapatch/issues/18),
[GH #19 manual recipe provenance](https://github.com/tesseracode/tesserapatch/issues/19),
[GH #20 legacy generation adoption](https://github.com/tesseracode/tesserapatch/issues/20),
[GH #21 doctor recipe migrations](https://github.com/tesseracode/tesserapatch/issues/21),
[GH #22 later-touch acknowledgement](https://github.com/tesseracode/tesserapatch/issues/22)

## 1. Scope

The downstream repository is a long-lived cumulative fork with 56 tracked
features. Its team reported that `verify --all` could no longer distinguish a
broken feature from ordinary historical drift, and requested migration support
for newer metadata and recipe contracts.

This assessment independently ran the published commands, inspected the
relevant artifacts and source contracts, and separated measured failures from
the downstream diagnosis. It did not edit or reset the downstream repository.
Its pre-existing 53 modified `status.json` entries produced an identical
`git status --short` listing before and after the verification and project
health gates. A separate repeated v0.15.1 `--no-write` run hashed every file
under `.tpatch/` before and after; the file set and all hashes were unchanged.

## 2. Reproduced results

The installed `tpatch v0.15.1` reproduced the report exactly:

```text
Summary: 0 passed, 53 failed, 3 skipped, 0 error
intent_files_present: 1
post_apply_patch_replay_clean: 38
recipe_replay_clean: 16
write_file_preimage_fresh: 6
```

The command exited 2. Repeating with the current `main` binary at `7206dab`
produced the same counts.

The downstream repository health claims also reproduced:

```text
bun run typecheck  PASS
bun run lint:all   PASS
bun test           352 pass, 0 fail
bun run build      PASS
```

`health-endpoint`, an old feature, fails
`post_apply_patch_replay_clean` in forward mode, supporting the claim that the
condition predates the August features.

## 3. Cumulative replay evidence

For all 38 V8 failures, a temporary Git index loaded each feature's
`status.apply.base_commit` and ran the canonical patch through
`git apply --cached --check`:

| Own recorded base result | Features |
|---|---:|
| Patch applies | 29 |
| Patch still fails | 9 |

This confirms a missing cumulative-stack verification/migration surface, but
also rejects a naive "`--own-base` means pass" solution. An isolated historical
apply does not prove effective-stack integrity, and nine features need a
different explanation or migration. GH #18 owns the contract research.

GH #18 must also explain all 16 V7 failures rather than treating V8 as the
whole population: 10 are direct forward recipe failures, two stop in parent
replay, two stop on parent-evidence integrity, and two stop on malformed
historical landing evidence. Those classes may need different repairs and
must not all be relabelled as cumulative drift.

`reconcile --cumulative-legacy` is relevant precedent, not an existing answer:
that flag controls multi-slug incremental derivation and does not change
verify's V7/V8 baselines or supply a migration assessment.

## 4. Preimage diagnosis correction

The shared-file pressure is real. Four consecutive feature commits changed
non-overlapping `README.md` regions, and recent features also repeatedly
touched `src/start.ts` and other shared paths.

However, the six reported V10 failures are not six measured stale hashes:

| Failure population | Count | Actual terminal reason |
|---|---:|---|
| Recent features | 4 | `recipe-provenance-unavailable` |
| Historical landed features | 2 | malformed landing evidence; no historical baseline |

Across the four recent failures, all 11 non-empty `preimage_hash` values match
the bytes at each feature's recorded `status.apply.base_commit`. Their
`artifacts/recipe-provenance.json` files are absent, so ADR-013 correctly
refuses to compare against the live working tree.

The producer gap is Path B:

- the shipped skill tells an agent to author a recipe, including preimages;
- `tpatch implement <slug> --manual` checks only file presence and JSON syntax;
- provider-backed `RunImplement` writes `recipe-provenance.json`, while the
  manual path does not.

Publication or adoption must also prove `recipe.feature` equals the requested
slug before writing provenance or state. Strict recipe decoding currently
requires only a non-empty owner, while later-touch and supersession severity
consume that owner; blessing a cross-feature recipe would be unsafe.

The accepted artifact-validation/provenance contract reserves Path B
tightening for a future PRD. Adoption also cannot reuse generation provenance
naively: the current sidecar and diagnostics claim a `generated_at` event,
while an historical recipe has no provable generation time. GH #19 therefore
requires an accepted generated/manual/adopted provenance contract before code.

Later-touch is already computed from metadata and emitted as a warning; it is
not what blocks these four V10 rows. GH #19 owns manual provenance publication
and safe adoption of already-provable recipes. GH #22 separately owns an
operator acknowledgement that cannot weaken replay safety.

## 5. Doctor and migration findings

Current read-only doctor output contains 30 drift findings and 55 warnings:

| Check | Count | Disposition |
|---|---:|---|
| D2 missing patch generations | 24 | Confirmed migration gap; GH #20 |
| D3 stale installed assets | 6 | Existing fixer available |
| D4 lock drift | 1 | Existing remediation |
| D5 evidence warnings | 53 | Informational population |
| D6 release drift | 1 | Existing remediation |
| D7 recipe schema drift | 0 now | Eight recipes were hand-migrated at downstream commit `578d189`; GH #21 |

The D2 remediation is not executable for unchanged historical patches:
`feature patch refresh` prints `no patch byte change; refresh skipped` and
does not create a manifest. ADR-024 D4 deliberately prohibited invented
historical backfill in v1, so GH #20 requires a new truthful representation,
not a bypass.

The report overstates which provenance is absent. All 24 D2 features have a
non-empty, reachable `status.apply.base_commit`, but that value is only
candidate evidence for a future generation record: six canonical patches
apply cleanly to it and 18 do not. Historical capture mode/pathspec selection
is still unavailable. Adoption must preserve the difference between recorded,
validated, inconsistent and unknown fields instead of discarding the candidate
or promoting it blindly.

D7 remains non-fixable in the CLI. The downstream migration was mechanical
(`version` removed, required `feature` added, operation semantics preserved),
which is suitable for an explicit guarded migration registry rather than
provider regeneration. Because even a mechanical transform changes recipe
bytes, a fixer must refuse a hash-bound recipe or atomically migrate/invalidate
every provenance, generation, verification and landing identity bound to the
old bytes. It must not silently leave an old hash presented as current.
`PRD-tpatch-doctor` is still Proposed and makes D7 read-only; it must be
revised and formally accepted (or replaced by another accepted authority)
before a fixer is implemented.

## 6. Request disposition

| Downstream ask | Assessment | Tracking |
|---|---|---|
| Verify cumulative forks or document migration | Confirmed gap; own-base probe recovers 29/38 V8 cases but is insufficient alone, and all 16 V7 failures need separate disposition | GH #18 |
| Distinguish legitimate later touch from corruption | Distinction already exists as warning versus baseline mismatch; reported recent blocks are missing manual provenance | GH #19 and GH #22 |
| Backfill missing patch generations honestly | Confirmed; current remediation cannot create same-byte manifests; all 24 have candidate bases but only 6/24 pass a compatibility probe | GH #20 |
| `doctor --fix` for recipe schema changes | Confirmed; downstream cleared eight with a bounded mechanical transform | GH #21 |
| Acknowledge modelled overlaps | Confirmed UX/audit gap; a dependency edge alone must not authorize unsafe overwrite | GH #22 |

Closed GH #1, GH #2 and GH #8 addressed adjacent safety and verification
defects, but none supplies these migration paths. Open GH #13 and GH #15 own
future semantic replay and recipe generation, not this historical adoption
population.

## 7. GitHub issue audit

Every issue from GH #1 through GH #17 was opened by `jdbencardinop`. The eleven
issues added after the user's earlier six-issue checkpoint are:

- GH #7 nested-worktree capture safety;
- GH #8 post-land verification;
- GH #9 the wave-close nested-source sentinel;
- GH #10 artifact-validation/provenance planning;
- GH #11 transactional prepare planning;
- GH #12–#15 absorption, semantic replay, reorder and recipe research;
- GH #16 read-only prepare-check implementation;
- GH #17 Windows full-suite parity.

No third-party issue requires adjudication.

## 8. Limits

- Passing a patch against its own base does not prove current stack
  correctness.
- This assessment did not re-record, reorder or reconcile the 56-feature
  stack.
- It verified the synthetic migration evidence and current contracts, not the
  semantic independence of every downstream feature.
- The new issues authorize planning only and do not preempt the accepted
  `implement-prepare-intent-bundle` queue head.
