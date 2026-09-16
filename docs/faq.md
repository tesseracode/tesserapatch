# FAQ

## Where is my config stored?

tpatch uses two config locations:

- **Global** (user-level): defaults, opt-in flags, Copilot AUP
  acknowledgement.
- **Repo** (`.tpatch/config.yaml`): per-repository overrides.

The repo values win over the global ones, except for a small set of
"global-only" keys (e.g. `provider.copilot_native_optin`) that must stay
user-wide so they don't accidentally follow a clone.

### Global config path by OS

| OS      | Resolved path                                               |
|---------|-------------------------------------------------------------|
| Linux   | `$XDG_CONFIG_HOME/tpatch/config.yaml` → `~/.config/tpatch/` |
| macOS   | `~/Library/Application Support/tpatch/config.yaml`          |
| Windows | `%AppData%\tpatch\config.yaml`                              |

### Why is macOS not `~/.config/`?

Go's standard library (`os.UserConfigDir()`) follows Apple's
conventions, which map user config to
`~/Library/Application Support/`, not `~/.config/`. This is intentional
— macOS apps have always lived there, and Spotlight/Time Machine honour
it.

If you prefer the XDG layout on macOS (e.g. you manage dotfiles across
Linux + macOS and want parity), set:

```sh
export XDG_CONFIG_HOME="$HOME/.config"
```

tpatch honours `XDG_CONFIG_HOME` on every platform. With that set,
macOS too will read/write `~/.config/tpatch/config.yaml`.

## Where is my Copilot auth stored?

The native Copilot provider stores a long-lived OAuth token at:

| OS      | Path                                                                 |
|---------|----------------------------------------------------------------------|
| Linux   | `$XDG_DATA_HOME/tpatch/copilot-auth.json` → `~/.local/share/tpatch/` |
| macOS   | `~/Library/Application Support/tpatch/copilot-auth.json` *†*         |
| Windows | not yet supported — use `copilot-api` proxy instead                  |

*†* macOS currently uses the same prefix because `os.UserConfigDir` and
`os.UserHomeDir`-based XDG fallback both land there. Set
`XDG_DATA_HOME=$HOME/.local/share` to force the Linux-style location.
The env var `TPATCH_COPILOT_AUTH_FILE` overrides the path entirely
(used by tests and unusual deployments).

The file is written with `0600` permissions. Run
`tpatch provider copilot-logout` to delete it.

## How do I opt in to the native Copilot provider?

```
tpatch config set provider.copilot_native_optin true
tpatch provider copilot-login
tpatch provider set --preset copilot-native
```

See [ADR-005](adrs/ADR-005-m11-native-copilot-provider.md) for the
security and policy rationale.

## How do I verify my provider works?

```
tpatch provider check
```

prints the configured endpoint and the list of models the server
reports. For the native provider this also triggers the session
refresh if needed.

## Does complete recipe coverage mean safe replay after an upstream update?

No. Recipe coverage is necessary, not sufficient, for future replay eligibility; it is not cross-base safety.
A warn/exit0 coverage row is not eligibility and never grants replay permission.

Current GH #15 implementation (v0.17 planned, unreleased) proves correspondence
to an immutable captured reference, not a new upstream base. Complete v1
admits only preimage-bearing `write-file` operations (including explicit-empty
creation), subject to all ten predicates. Append, replacement and ungated
writes are not complete v1 operations. GH #13's new reconcile replay consumer
is future separate-release work; landing/attestation remains independent.
See [the producer and proof contract](../SPEC.md#recipe-generation-authority-gh-15-v017-planned-unreleased).

## Why is verify green when coverage is missing, incomplete or marked stale?

The `recipe_generation_coverage` check is warning-class for valid incomplete
coverage, a stale marker beside valid complete coverage, or genuinely absent
coverage. These rows leave verify passed/exit 0 **absent other failures**.
Pre-v0.17 features with no coverage, including old `recipe-stale.json`,
therefore remain compatible. Present malformed coverage or stale bindings
instead block at exit 2.

`recipe-capture-event.json` is atomically published before
`recipe-coverage.json`, not as a cross-file transaction. It is unkeyed
consistency evidence, not authentication/history; readers reconstruct the
proof. An orphan event beside absent coverage still follows legacy behavior.

## Will doctor --fix or record always repair a recipe?

No. `tpatch doctor --check D10 --fix` is still read-only, warning-only and
never repairs artifacts. It recommends `record <slug> --regenerate-recipe`
only after a read-only plan of that exact default command establishes
complete D16-proven output and passes the real producer gates. Otherwise
`recipe-generation-no-truthful-regeneration` explains the blockers.

D16 requires full freshly derived canonical-byte equality, not matching
paths, labels or historical provenance. Differing manual/provider recipes
are preserved unless complete regeneration is explicitly authorized.
Formatting-only mismatch can leave a stale marker without semantic rewrite
reasons. Unsupported effects withhold a new partial recipe; regeneration
does not widen support.

If ordinary execute refuses `recipe-generation-incomplete` (exit 2), coverage
binds no readable executable recipe. An already-applied feature should be
inspected with `verify`/`status`. Other states require review before explicit
manual patch application or authoring/checkpointing a recipe; `implement
--manual` moves state to `implementing`. See [apply recovery](../SPEC.md#explicit-apply-and-ordered-no-write-success).
State-selected canonical-patch reapply is separate: no `--reapply` flag and
no `--mode reapply` exist.
