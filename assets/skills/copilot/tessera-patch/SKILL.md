# Tessera Patch — GitHub Copilot Skill

## Overview

Tessera Patch customizes open-source projects through natural-language patches while maintaining upstream compatibility.

## CLI Commands

- `tpatch init` — Initialize workspace
- `tpatch add <description>` — Create feature request
- `tpatch status` — Feature dashboard
- `tpatch analyze <slug>` — Run analysis
- `tpatch define <slug>` — Generate acceptance criteria
- `tpatch explore <slug>` — Explore codebase
- `tpatch implement <slug>` — Generate apply recipe
- `tpatch apply <slug>` — Execute recipe or record session
- `tpatch record <slug>` — Capture patches
- `tpatch reconcile` — Reconcile with upstream
- `tpatch provider check` — Validate provider
- `tpatch config show|set` — Manage config

## Lifecycle

analyse → define → explore → implement → test → record → reconcile

## Data Model

Features are tracked in `.tpatch/features/<slug>/` with `status.json`, `request.md`, `analysis.md`, `spec.md`, and `artifacts/`.

## Reconciliation

4-phase decision tree: reverse-apply → operation-level → provider-semantic → forward-apply.

Outcomes: `upstream_merged`, `reapplied`, `blocked`.
