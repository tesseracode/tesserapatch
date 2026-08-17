# Current Handoff

## Status

**Cluster state**: AWAITING REVIEW

The accepted read-only `tpatch prepare <slug> --check` implementation is
dispatched as one sequential wave. No mutating prepare mode is authorized.

## Active Task

- **Task ID**: `implement-prepare-check`
- **Issue**: [GH #16](https://github.com/tesseracode/tesserapatch/issues/16)
- **Description**: Implement
  `PRD-artifact-validation-and-provenance` rev-5 + ADR-034 rev-2.
- **Status**: Awaiting Review
- **Assigned**: 2026-08-17
- **WAVE_BASE**: `9a8c1d049bb973ccf377bd9f0fa67d7080d2d773`
- **Release tag**: none assigned; this prerequisite must be reviewed before any
  mutating prepare release planning

## Contract

- Command: `tpatch prepare <slug> --check`.
- Read only: no provider, prompt, lock, filesystem mutation, state transition,
  status timestamp, `FEATURES.md` refresh, or artifact write.
- Required artifacts: `analysis.md`, `spec.md`, `exploration.md`.
- Optional analysis sidecar never affects readiness.
- Full accepted nine-state artifact classification and three-document readiness.
- Constant `provenance: unknown`.
- One held Go 1.26 `*os.Root`; logical root confinement, unsafe/non-regular
  refusal, bounded `MaxArtifactBytes+1` reads, honest instability reporting.
- Accepted `unix || windows` fail-closed platform policy.
- Exact human/JSON/quiet output, exit codes and precedence from the 208-row
  `AVP-001…AVP-208` matrix.
- Existing per-phase `--manual`, `next`, `cycle`, and `apply --mode prepare`
  behavior stays byte-compatible.

## Scope and Discipline

- One sequential implementer owns the wave because CLI registration, output
  contracts, tests and public docs overlap.
- Explicit-path staging only; no broad add/commit.
- Implementation goldens must come from this wave's commit range, not from the
  later mutating-prepare cluster.
- Native Windows behavior is part of acceptance.
- Mutating prepare PRD rev-14 / ADR-035 rev-14 remains blocked.

## Session Summary

- Read the required implementation contracts: `CLAUDE.md`, `AGENTS.md`,
  PRD-artifact-validation-and-provenance rev-5, and ADR-034 rev-2.
- Implemented the read-only `prepare <slug> --check` CLI, rooted inspection
  core, deterministic JSON/human/quiet report renderers, exit precedence, and
  status/artifact capture ladders.
- Added supported/unsupported platform build tags, native Windows CI coverage,
  the `winsymlink=1` main-package directive, targeted unit/integration tests,
  docs, and all six skill-surface updates.
- The accepted contracts require
  an implementation correction: the documented bare constant subtraction does
  not reject an inverted cap relationship, so the implementation uses a real
  negative-array-length compile-time guard. This is an implementation erratum,
  not a product-contract change.

## Current State

- `prepare --check` is a pure `internal/intent.Inspect` call over a three- and
  three-method rooted operation seam. It uses one caller-owned 4 MiB+1 scratch
  buffer sequentially for `status.json` and all four artifacts.
- The command opens one root after workspace discovery and closes it after
  report rendering. It makes no writes, provider calls, subprocess calls, or
  lifecycle changes.
- Existing `apply --mode prepare`, manual phase gates, `next`, and `cycle`
  are unchanged; apply help has the reciprocal collision pointer.
- Known allowlisted untracked research WIP remains untouched.

## Files Changed

- `.github/workflows/ci.yml`
- `CHANGELOG.md`, `SPEC.md`
- `assets/assets_test.go`
- `assets/prompts/copilot/tessera-patch-apply.prompt.md`
- `assets/skills/claude/tessera-patch/SKILL.md`
- `assets/skills/copilot/tessera-patch/SKILL.md`
- `assets/skills/cursor/tessera-patch.mdc`
- `assets/skills/windsurf/windsurfrules`
- `assets/workflows/tessera-patch-generic.md`
- `cmd/tpatch/main.go`
- `docs/agent-as-provider.md`, `docs/feature-layout.md`,
  `docs/path-b-operator-guide.md`, `docs/handoff/CURRENT.md`
- `internal/cli/cobra.go`, `internal/cli/prepare.go`,
  `internal/cli/prepare_test.go`
- `internal/intent/intent.go`, `internal/intent/inspect.go`,
  `internal/intent/render.go`, `internal/intent/confine_supported.go`,
  `internal/intent/confine_unsupported.go`, `internal/intent/openflags_unix.go`,
  `internal/intent/openflags_windows.go`, `internal/intent/openflags_unsupported.go`,
  `internal/intent/inspect_test.go`

## Test Results

- Targeted assets/intent/CLI tests: PASS (5 top-level tests plus 14
  table-driven subcases).
- Asset parity: PASS.
- `gofmt -l .` and `git diff --check`: PASS.
- `go vet ./...`: PASS.
- `go build ./...` and `go build ./cmd/tpatch`: PASS.
- `go test -count=1 ./...`: PASS (14 test packages; 2 command packages
  correctly have no tests).
- Cross-build/vet: `GOOS=windows GOARCH=amd64 go vet ./internal/intent
  ./internal/cli`, Windows CLI build, and `GOOS=wasip1 GOARCH=wasm` CLI build:
  PASS. `wasip1` compiles but is refused by the runtime platform guard.

## Next Steps

1. Reviewer: compare output text, report schema/order, and every ladder row
   against PRD rev-5; focus especially on close precedence and abort ordering.
2. Reviewer: run the accepted matrix/guard pass, including native Windows
   behavior and unsupported-target refusal.
3. If approved, commit this explicit file list and push `origin/main`.

## Blockers

- No implementation blocker.
- Hard block for every mutating prepare slice until this wave is accepted and
  landed.

## Context for Next Agent

- The `openflags_unsupported.go` zero value exists only to keep unsupported
  targets buildable; the CLI refuses before root opening.
- Do not modify the accepted PRD/ADR, ROADMAP, supervisor LOG, HISTORY, or
  research WIP. The untracked research files visible in `git status` predate
  this wave and are not task files.
- Reviewer should preserve the negative-array-length cap guard; the former
  bare subtraction would not enforce the intended strict ordering.
