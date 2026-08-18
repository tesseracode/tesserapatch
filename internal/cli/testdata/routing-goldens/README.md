# Pre-change routing goldens — provenance

These fixtures satisfy `PRD-artifact-validation-and-provenance` rows
**AVP-071**, **AVP-072**, **AVP-136** and **AVP-137**. Those rows require a
comparison against the **pre-change** binary, not a before/after comparison
across a no-op run of the current binary: a routing change introduced by this
wave would appear on both sides of a same-binary comparison and be invisible.

## Recorded from

| Field | Value |
|---|---|
| Wave | GH #16 — `implement-prepare-check` rev-1 |
| `WAVE_BASE` commit | `9a8c1d049bb973ccf377bd9f0fa67d7080d2d773` |
| Commit subject | `docs: accept semantic replay and absorption research` |
| Recorded on | 2026-08-17 |
| Toolchain | `go version go1.26.5 darwin/arm64` |
| Baseline binary SHA-256 | `c06c205cc8a819aa8bb4e10eb8542c4b5174793920cfcec56b1b57d2d8388de5` |

The baseline binary was reconstructed in a **temporary detached worktree** at
`WAVE_BASE` and deleted after recording — no build artifact is tracked:

```sh
git worktree add --detach .golden-baseline 9a8c1d0
(cd .golden-baseline && go build -o ../.golden-baseline-bin/tpatch-9a8c1d0 ./cmd/tpatch)
TPATCH_ROUTING_GOLDEN_BIN="$PWD/.golden-baseline-bin/tpatch-9a8c1d0" \
  TPATCH_RECORD_ROUTING_GOLDENS=1 \
  go test ./internal/cli -run TestAVPRoutingGoldens
git worktree remove .golden-baseline && rm -rf .golden-baseline-bin
```

A normal `go test ./internal/cli -run TestAVPRoutingGoldens` run builds the
**current** binary from `./cmd/tpatch` and compares every file below
byte-for-byte.

## Manifest

| File | Population | Row |
|---|---|---|
| `next-requested-text.txt` | `next <slug>`, state `requested`, `text` | AVP-071, AVP-136 |
| `next-requested-harness-json.txt` | `next <slug>`, state `requested`, `harness-json` | AVP-071, AVP-136 |
| `next-analyzed-text.txt` | `next <slug>`, state `analyzed`, `text` | AVP-071, AVP-136 |
| `next-analyzed-harness-json.txt` | `next <slug>`, state `analyzed`, `harness-json` | AVP-071, AVP-136 |
| `next-defined-pre-explore-text.txt` | `next <slug>`, `defined` before explore, `text` | AVP-071, AVP-136 |
| `next-defined-pre-explore-harness-json.txt` | `next <slug>`, `defined` before explore, `harness-json` | AVP-071, AVP-136 |
| `next-defined-post-explore-text.txt` | `next <slug>`, `defined` after explore, `text` | AVP-071, AVP-136 |
| `next-defined-post-explore-harness-json.txt` | `next <slug>`, `defined` after explore, `harness-json` | AVP-071, AVP-136 |
| `next-apply-mode-prepare.txt` | `apply <slug> --mode prepare` — the surface that shares the word `prepare` (§5.2) | AVP-136 |
| `cycle-skip-execute-transcript.txt` | `cycle <slug> --skip-execute` on a heuristic (no-provider) workspace | AVP-072, AVP-137 |
| `cycle-final-state.txt` | the same run's final `status.json` state line | AVP-072, AVP-137 |
| `changed-apply-help.txt` | `apply --help` **before** this wave | AVP-010 |

`changed-apply-help.txt` is deliberately **excluded** from the byte-identity
rows. `apply --help` is the one pre-existing surface this wave is authorized to
change — AVP-010 requires the `--mode` description to point at
`tpatch prepare <slug> --check`. `TestAVPRoutingGoldens/AVP-010-bounded-delta`
asserts the current output equals the recorded pre-change output with exactly
that one sentence appended, so the change stays bounded and reviewable.

## Normalization

Each file is `$ tpatch <args>`, the process exit code, and the combined
stdout+stderr with the workspace's absolute temp path replaced by
`<workspace>`. Nothing else is rewritten; the bytes are otherwise verbatim.
