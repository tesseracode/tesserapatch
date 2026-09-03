# Promoted fixture — adjacent CLI argument conflict

**Source**: `docs/state-of-the-art/case-studies/adjacent-cli-args-conflict-2026-08/`
(`reproduce.sh` `after` / `delete-all` variant, and `reproduce-tpatch.sh`).

**Promoted by**: GH #15 / ADR-036 S0 frozen evidence
(`docs/prds/PRD-recipe-generation-authority.md` §8 "S0 - Frozen evidence":
"Promote the adjacent-conflict scripts and downstream V10 case into fixtures.").

These bytes are a stable, in-repo copy of the shell-script fixture so the
baseline can be asserted from Go without `mktemp`, `sed` or a network. They
freeze CURRENT behaviour only. No fix is implemented against them.

| File | Role |
|---|---|
| `base.command.go.txt` | the shared base: two upstream arguments |
| `feature.command.go.txt` | the feature tree: two feature arguments added **after** the upstream ones |
| `upstream.command.go.txt` | the upstream tree: both upstream arguments intentionally deleted |
| `resolved.command.go.txt` | the semantically correct resolution the case study names |
| `feature.post-apply.patch` | real `git diff` of base → feature, captured with Git's default settings; context-marker spaces are stored as `{{CONTEXT}}` and restored by the fixture loader so `git diff --check` does not misread valid space+tab patch lines as source indentation |

`.txt` suffixes keep `gofmt -l .` and every Go tool away from the snippets;
`testdata/` is already ignored by the Go tool itself.

## What the case study recorded, and what the S0 tests freeze

1. Recipe autogeneration emits a **whole-file `write-file`** operation with no
   `preimage_hash` (`summary.md` §3.1, §3.3).
2. Replayed against the upstream tree, that whole-file operation **restores the
   intentionally deleted upstream arguments** (`summary.md` §2.6 of the PRD).
3. A hand-authored `replace-in-file` operation anchored on the slice closing
   brace is **applicable** on the upstream tree, yet current phase 2 recognises
   only the `allPresent` case and appends a note only when an operation
   conflicts — so an applicable-only operation is neither replayed nor surfaced
   (`summary.md` §3.2).
4. `replace-in-file` is **not idempotent** and uses first-substring matching
   with no anchor-uniqueness requirement (`summary.md` §3.3).
5. A whole-file `write-file` whose target is missing is treated as applicable
   and can **resurrect an intentionally deleted file** (`summary.md` §3.3).
