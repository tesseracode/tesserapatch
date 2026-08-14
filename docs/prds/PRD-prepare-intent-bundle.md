# PRD — Prepare Intent Bundle — `tpatch prepare <slug>` (mutating modes)

**Status**: Draft — Awaiting Review (rev-11)
**Date**: 2026-08-14
**Owner**: Core (planning lane)
**Byline**: sole sequential planning writer, rev-11 from reviewed writer tip `a9ad7c0`;
dispatch/base `c394117`; WAVE_BASE `d060ff4`
**Milestone**: TBD — this document ships no code
**Issue**: [GH #11 — define the mutating prepare intent-bundle contract](https://github.com/tesseracode/tesserapatch/issues/11)
**Graduates from**: [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md), Turns 2–4
**Prerequisite (accepted)**: [PRD-artifact-validation-and-provenance](./PRD-artifact-validation-and-provenance.md)
rev-5 (Accepted 2026-08-13) and [ADR-034](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md)
rev-2 (Accepted 2026-08-13). This PRD builds **on top of** that read-only
contract and does not reopen it. Its **implementation** is a hard sequencing
prerequisite for every mutating slice here (§17.1).
**Architecture**: [ADR-035 — Intent bundle publication and history](../adrs/ADR-035-intent-bundle-publication-and-history.md)
(**Proposed**, rev-11). **This PRD and ADR-035 must be reviewed together.**
ADR-035 locks the publication/history decisions; this PRD states the product
contract that depends on them. Where they overlap, **ADR-035 is normative**.

> **Implementation is not authorized by this document.** No Go file, test,
> asset or CLI surface may change until **both** this PRD and ADR-035 are
> accepted. §19 states the gate; §17.2 states the slices that become dispatchable
> afterwards.

## Related

- [PRD-artifact-validation-and-provenance](./PRD-artifact-validation-and-provenance.md) — the accepted read-only `prepare <slug> --check` contract this PRD extends without modifying (its §20 lists exactly what this document must answer)
- [ADR-034 rooted filesystem inspection boundary](../adrs/ADR-034-rooted-filesystem-inspection-boundary.md) — D1–D18; reused verbatim for every canonical **read** this command performs
- [ADR-035 intent bundle publication and history](../adrs/ADR-035-intent-bundle-publication-and-history.md) — **companion, Proposed rev-11**; the transaction, the archive and the honesty limits
- [WP-005 Spec-driven workflows](../whitepapers/WP-005-spec-driven-workflows.md) — `## Agreed — Turns 2–3` items 4–9
- [WP-005 turn log](../whitepapers/WP-005-spec-driven-workflows.turns.md) — Turns 2, 3 and 4
- [Agent as Provider — Path B workflow](../agent-as-provider.md) — the phase → artifact → state contract
- [Path B Operator Guide](../path-b-operator-guide.md) — the hand-authored artifact flow this PRD's `--manual` mode adopts
- [Feature Layout](../feature-layout.md) — canonical vs audit-trail files under `.tpatch/features/<slug>/`
- [PRD-tpatch-land](./PRD-tpatch-land.md) — the shipped journal + crash-recovery precedent this PRD reuses and deliberately diverges from (redo vs undo)
- [PRD-feature-resource-claims-and-capture-adapters](./PRD-feature-resource-claims-and-capture-adapters.md) — the shipped content-addressed immutable-set + pointer precedent the archive reuses, and the shipped kernel `flock(2)` process-lifetime **precedent** this PRD's directory authority follows without extracting or reusing it (`internal/rescap/lock_unix.go:6-11`; §7.4.4, §17.1)
- [ADR-033 resource capture boundary](../adrs/ADR-033-resource-capture-boundary.md) — D4 (the closed six-class redaction scan), D10 (no tracked timestamps), D11 (no Go map in a wire schema)
- [ADR-027 capture context privacy boundary](../adrs/ADR-027-capture-context-privacy-boundary.md) — **D3 (redaction is a write precondition, not best-effort cleanup)**, D2 (no raw context), D5 (retention semantics), D6 (no wall-clock in determinism)
- [ADR-031 rejected feature state data model](../adrs/ADR-031-rejected-feature-state-data-model.md) — the per-command exit envelope and closed-enum precedent
- [PRD-active-feature-session](./PRD-active-feature-session.md) — the shipped `session purge` deletion grammar (dry-run default + `--yes`) reused by the archive retention surface (§9.7)
- [PRD-tpatch-doctor](./PRD-tpatch-doctor.md) — the diagnostic surface that gains the pending-transaction / orphan-state check (§12.5)

## Revision history

| Rev | Disposition | What changed |
|---|---|---|
| rev-0 | Draft — Awaiting Review | First draft. Defines the four modes, the publication transaction, the undo journal, the content-addressed intent archive (**architecture gate fired → ADR-035**), the staged generator extraction, the lifecycle/compatibility deltas and a 234-row acceptance matrix. |
| rev-1 | Draft — Awaiting Review | Closes every rev-0 finding. **Lock authority replaced**: the unprovable `O_EXCL` stale-lock model is gone; rev-1 **proposed** reusing the shipped kernel `flock(2)` process-lifetime lock by extraction, plus a Windows deny-share sibling, so ownership would be released on process death and liveness would be decidable without a journal (rev-1 §7.4, ADR-035 D4 as it then stood). *That extraction proposal was superseded: rev-2 and rev-3 replaced it with a prepare-owned held-root directory authority, and no `rescap` extraction is proposed by any later revision.* **Journal rebuilt**: plan-digest binding, old *and* intended-new identity for every canonical **and** metadata path, and a semantic compare-and-swap before every publish, undo and remove (§7.5, §7.6, §7.9, ADR-035 D5). **T1 scoped honestly** to a command-owned verification point under the lock (§7.1, ADR-035 D6). **Rooted writes**: `(*os.Root).Rename` / `(*os.Root).OpenFile` adopted for every publication write, so the residual race narrows to content identity (§7.7, ADR-035 D2). **Default coherence**: the sidecar is never overwritten without `--regenerate`, and the generated set must be a dependency-coherent suffix (§6.1). **Archive privacy/retention**: ADR-027 D3's redaction scan is a write precondition and a match refuses regeneration; the "no new exposure class" claim is withdrawn; a bounded `tpatch feature intent-archive list\|purge` retention surface, tombstones and orphan-blob reporting are added (§9, ADR-035 D8/D15/D16). **`--regenerate` requires a configured provider** unless `--allow-heuristic` is passed (§11.3, ADR-035 D18). **Git usage disclosed and made conditional** (§7.13, ADR-035 D17). **`FEATURES.md` named as derived, best-effort and outside T1** (§12.3.1). **Dry-run refuses when recovery is pending** (§6.4). **Exit 6 gains a safe `--abandon-transaction` route** (§6.6). Matrix rebuilt to 394 rows (160 new, 15 amended in place). |
| rev-2 | Draft — Awaiting Review | Adjudicates every rev-1 finding without reopening prior closures. **Stable lock authority** moves outside the worktree to a persistent, opaque per-user cache namespace keyed by canonical native workspace-root identity; Git cleanup cannot unlink/recreate a live authority (§7.4, ADR-035 D4). `--manual` is now a rooted single-file temp+rename with an immediate status preimage CAS (§6.2, ADR-035 D2/D3). Tombstones retain immutable content digests, X10 validates them after purge, and a duplicate tombstoned generation rehydrates rather than leaving an orphan (§9.3, §9.7, ADR-035 D10/D16). Purge obtains a strict index preimage and CASes it immediately before rename. Mutating support is honestly `linux`, `darwin`, `windows`; read-only `--check` remains `unix \|\| windows`. G1, run from the workspace root, replaces `.git` probing; only Git-established non-worktrees may use the non-Git route (§7.13). Rooted writes stop outside-root escape but can follow an in-root redirect; CAS detects only an identity mismatch, and the exact residual is disclosed (§7.7, ADR-035 D2/D6). Doctor no longer infers journal loss from ordinary lifecycle bytes. Matrix: 409 rows, with 15 new rev-2 rows (`PIB-395`…`PIB-409`) and the listed amended rows. |
| rev-3 | Draft — Awaiting Review | Adjudicates the rev-2 review without reopening earlier closures. **D4 is subtractive:** one held workspace `*os.Root` supplies `Root.Open(".")`'s directory descriptor and its single nonblocking `flock`; there is no cache, key, lock pathname, user-cache/home lookup, Windows implementation, cleanup residue or slug partition. Mutation is Linux/Darwin-only and serializes every prepare mutation and archive purge in the workspace. Root identity is taken from that descriptor and compared to the re-resolved live root before publication and at final verification. Raw provider text is never persisted, even locally: retry keeps only bounded in-memory bytes long enough to validate/render and optional redacted/hashed metadata. Purge truth now covers the non-atomic CAS→rename/remove tail, global shared-hash rehydration, dangling blobs, `git clean` deletion of untracked archives, and a scrubbed single-G1 Git seam. Matrix: 432 rows; rev-3 amends listed stable rows and adds `PIB-410`…`PIB-432`. |
| rev-4 | Draft — Awaiting Review | Bounded adjudication of rev-3 implementation contradictions. An authority retains both the workspace `*os.Root` and the locked directory `*os.File` through explicit unlock/close with `runtime.KeepAlive`; no naked fd or finalizer releases a live lock. Root rename no longer claims rediscovery success. The archive has one per-hash pending → remove/recover → tombstone state machine, including repairable dangling live references. Dry-run branches after read inspection and before every mutating gate, Git call and lock. The Git seam is a closed read-only argv allowlist with a single G1 result threaded to relative-path helpers. Root-filesystem policy is a denylist plus real flock, scoped only to the locked root inode. Doctor gains a non-creating lock probe. Raw-response guards are structural rather than impossible byte-substring scans. Matrix: 448 rows; rev-4 amends listed stable rows and adds `PIB-433`…`PIB-448`. |
| rev-5 | Draft — Awaiting Review | Bounded reachability and totality adjudication of rev-4; no product choice is reopened. **Abandon is reachable**: `--abandon-transaction` branches after the mutating gate and the lock and **before** journal and archive recovery, so it can never auto-recover the evidence it was invoked to preserve; corrupt, divergent and recoverable journals all abandon successfully (§6.6, §7.8, §7.11, §10.5). **Every control write is rooted**: a prepare-owned rooted durable single-file helper replaces path-based `gitutil.DurableWriteFile` for the journal, both raw preimages, staging and every `.tpatch/local/` write; `DurableWriteFile`/`writeFileAtomic` are shape precedents only (§7.5, §7.7.1, §13.2, ADR-035 D2). **One dangling repair**: unreachable exact-content rehydration is removed as a dangling-reference remedy; the sole shipped repair is confirmed `tpatch feature intent-archive purge <slug> --blob <hash> --yes`, and the existing global rehydration path applies only to tombstoned/pending references afterwards (§9.3, §9.3.1, §9.7). **Dry-run states its exact scope**: it reproduces only the refusal classes it evaluates, enumerates the non-evaluated ones, always reports `execution_preflight: not_evaluated`, and its precedence branch runs every non-mutating gate before returning (§6.4, §10.5). **Partial purge is a distinct retryable outcome**: everything predictable is preflighted, and a failure after the first per-hash mutation is exit 5 `archive-purge-partial` with completed/pending/remaining hashes and an exact same-command retry; exit 3 keeps its zero-write promise (§9.7.2, §10.4). **Doctor stops probing**: D9 reports persistent evidence only and never opens or flocks the root (§12.5). **The Git slice is complete**: `internal/rescap/gitgate.go` joins the authorized central-gate refactor, the environment scrub list is pinned, and global/system ignore configuration is deliberately preserved (§7.13, §17.2). **Guards are feasible**: provenance guards inspect controlled schema keys and persistence sinks rather than `.tpatch` bytes (§9.8, §13.4). Matrix: 482 rows; rev-5 amends listed stable rows and adds `PIB-449`…`PIB-482`. |
| rev-6 | Draft — Awaiting Review | Final bounded ordering/grammar adjudication of rev-5; no product choice is reopened. **Recovery is terminal**: a successful automatic journal recovery or pending-purge recovery ends the invocation at exit 0 with `outcome: "recovered"`, reports what it restored or finalized, and hands the operator a sanitized equivalent retry; it never continues into the requested operation's later exit-2/exit-3 gates, which is what makes exit 3's zero-write promise unqualified (§7.8, §7.11, §10.4, §10.5). **Purge refuses a pending journal**: `intent-archive purge` never recovers, decodes or consumes a prepare journal — it refuses `recovery-pending` with the whole tree unchanged, and recovers only a pending *purge* state, terminally (§9.7, §9.7.2, ADR-035 D16). **Abandon is reachable through a broken Git**: it still requires the supported platform, the classified root filesystem and the exclusive root lock, but it bypasses G1–G4 and the local-lane privacy gate because it only moves existing bytes inside the same lane; where the environment denies the lock outright, the refusal names the repo-relative lane and a safe last-resort manual removal, and every “never terminal” claim is qualified by that route (§6.6, §7.8, §10.4, §10.5). **Retries are sanitized**: no retry or remediation echoes an inherited absolute `--path`; every retry is the equivalent command without root-selection argv plus the closed field `retry_cwd: "workspace-root"`, and a second abandon over `abandoned-*` residue alone refuses `no-pending-transaction` while preserving it (§9.7.2, §10.2, §10.4.1). **Flag grammar is honest**: default-mode `--allow-heuristic` is a legal no-op with an advisory in §5.1 as well as §5.2, and `--yes` outside `--abandon-transaction` is rejected by the command's own preflight at §10.5 step 1a with one fixed literal, because cobra cannot express a mutex against the *absence* of a mode flag. **Descriptor discipline is uniform**: `fstatfs` runs inside the retained directory file's `SyscallConn().Control` callback, and no `(*os.File).Fd()` appears in the authority path (§7.4.2). Matrix: 505 rows; rev-6 amends listed stable rows and adds `PIB-483`…`PIB-505`. |
| rev-7 | Draft — Awaiting Review | Bounded totality fold of rev-6; no product choice is reopened. **Exit-6 remediation is partitioned by population**: the nine journal/publication codes keep `--abandon-transaction`, and `archive-purge-evidence-divergent` — which abandon provably cannot consume, because that mode touches no index and no blob — gets its own repo-relative, executable archive procedure (preserve the unexpected bytes if wanted, remove the divergent managed blob path or restore the index, then rerun the sanitized purge, whose pending+absent recovery terminally tombstones), with its destructive cost and the Git-history caveat stated (§6.6, §9.7.2, §10.4, §10.4.1). **The pre-abandon gate is total**: rev-6's “exactly three conditions” is replaced by a total table over parse/flag, slug, workspace, read platform, mutating platform, root filesystem, contention and lock failure; contention says wait and retry rather than offering deletion under a live holder; and abandon is confirmed to branch **without** reading the feature directory or `status.json` and without G1–G4, so an absent, malformed or unreadable feature cannot block local evidence inspection (§6.6, §10.5 steps 6 and 10). **Lane parity**: PIB-283 is amended to the normal mutating modes and explicitly excludes abandon, dry-run, check, list and purge (§18.1, ADR-035 D17). **Partial purge branches**: the resumption story is conditional — pending hash present (recovery run then completion run), failure exactly between hashes (one completion run, no `recovered` promised) and `--orphans` (one rescan run, no `recovered` promised) — carried by the closed `purge_progress.resume` field (§9.7.2, §10.2, §10.4). **Pending-purge preview is honest**: with no journal and a pending hash, preview acquires no lock, writes nothing, recovers nothing, and says that recovery is required and that `--yes` from the workspace root performs it (§9.7). **Purge runs zero Git**: it writes the tracked archive and never the gitignored staging lane, so the lane gate has no subject there and G1 is scoped to normal mutating `prepare` alone (§7.13, ADR-035 D17). **`--allow-heuristic` joins the default grammar line** without changing its no-op/advisory semantics (§5.1). Every “never permanently blocked” sentence is qualified by the command or manual escape actually available, and the rev-6 amendment ledger is corrected to the rows it truly changed. Matrix: 520 rows; rev-7 amends listed stable rows and adds `PIB-506`…`PIB-520`. |
| rev-8 | Draft — Awaiting Review | Final bounded archive-state correction of rev-7; no product choice is reopened. **A tombstone beside a present blob is physical residue, not purge divergence**: it holds no pending reference, so it cannot be the subject of `archive-purge-evidence-divergent` or of the pending+absent escape. X11 classifies it as exit-3 `archive-index-storage-inconsistent` **unreferenced physical residue**, zero-write on discovery, and its one shipped repair is `tpatch feature intent-archive purge <slug> --orphans --yes` run from the workspace root, which is explicitly admitted past that X11 refusal after the strict X1–X10 wire decode and removes the unreferenced blob without rewriting the index; the preview reports it (§9.3, §9.3.1, §9.7.3, §10.4.1, ADR-035 D10/D16). Archive divergence keeps only genuinely pending-hash/index evidence and its manual `rm`/restore-then-rerun route. **Pending purge recovery is purge-owned**: a normal mutating `prepare` never runs `RecoverPendingPurge`; observing `purge_pending` in the index it refuses exit 3 `recovery-pending` with zero writes and names the sanitized `tpatch feature intent-archive purge <slug> --all --yes` with `retry_cwd: "workspace-root"`, so one command owns the pending-purge transaction and `archive-purge-evidence-divergent` has one owner (§7.8 step 5, §7.11, §10.4, §10.5 step 13). “After the first per-hash mutation” is scoped to the purge transaction in flight, which may have begun in a **prior** invocation. **Selector totality**: every `--yes` selector, `--orphans` included, finalizes any pending hash first and returns terminal `recovered`, after which the operator reruns the requested selector; every preview selector takes no lock, writes nothing and reports the pending recovery, and every emitted retry preserves the operator's own selector verbatim minus root-selection argv (§9.7, §9.7.1, §9.7.2). **The pending-purge preview has a bound shape**: the new closed outcome token `recovery-required` and the closed `pending_purge` object carry `recovery_required`, the pending hashes with their safe repo-relative blob/index paths and per-hash plan, the selector, the sanitized `retry` and `retry_cwd`, with asserted human/JSON parity (§10.2). **The pre-abandon gate table is total over reachable stops only**: the intentionally unreachable `--yes` row is removed and the table is defined over the gates a syntactically valid abandon invocation can actually reach (§6.6). Purge preview joins the never-acquire population explicitly, `--yes` purge acquires, and purge stays zero-Git in both forms (§7.4.1, §7.12). Matrix: 530 rows; rev-8 amends listed stable rows and adds `PIB-521`…`PIB-530`. |
| rev-9 | Draft — Awaiting Review | Final bounded global-hash and parity fold of rev-8; no product choice is reopened. **A tombstone beside a present blob is classified globally, not per reference**: X11 computes whether the hash is still live — any reference to it retained or removal-pending, in any generation — before deciding. Unreferenced ⇒ physical residue, `list` exit 0 rendering `orphan`, repaired by `tpatch feature intent-archive purge <slug> --orphans --yes`. Still live ⇒ a **mixed tombstone/live-reference global-availability inconsistency**: the blob is live data, `--orphans` excludes it and may never remove it, `list` exits 3 rendering `mixed-reference`, and the repair is the confirmed `tpatch feature intent-archive purge <slug> --blob <hash> --yes`, which selects every reference to the hash, runs the ordinary pending/removal/tombstone machine and makes global availability truthful. rev-8's per-reference rule would have deleted a retained reference's blob through the one selector that promises to remove nothing referenced (§9.3, §9.3.1, §9.7, §9.7.1, §9.7.3, §10.4.1, §12.5, §7.10 CP13, ADR-035 D10/D16). Neither subcase is `archive-purge-evidence-divergent`, which keeps exactly the pending-hash and pending-index evidence. **Compound ordering is fixed**: a pending hash plus a residue observation previews without a lock, recovers terminally on `--yes`, and the rerun then performs the correct subcase repair — with no dead end, since even the off-selector refusal names an executable command (§9.7.2). **One verbatim retry heading**: every emitted rerun prints `Run this again from the same workspace root:` byte-for-byte and nothing else; rev-8's `to perform it:` variant and the numbered divergence step are withdrawn, and PIB-498 becomes the derived totality guard over every emitter (§9.7.2, §10.2). **The pre-abandon gate table's domain is every argv that attempts to select `--abandon-transaction`**, parsing or not, which makes its parse/arity/mutex row reachable and removes rev-8's “syntactically valid” contradiction (§6.6, §10.5 step 10). **`os.OpenRoot` and `root.Open(".")` failures are classified** as `directory-flock-unavailable`, exit 3, before any `fstatfs` or `flock`, and carry gate row 8's manual route (§7.4.1, §6.6). **§6.2 discloses** that `--manual`, which writes no archive, is nonetheless refused zero-write by a removal-pending index and names the purge route. Matrix: 536 rows; rev-9 amends listed stable rows, re-kinds `PIB-498` from `I` to `G`, and adds `PIB-531`…`PIB-536`. |
| rev-10 | Draft — Awaiting Review | Final bounded ownership/scope fold of rev-9; no product choice is reopened. **A removal-pending reference makes the purge transaction the global owner of its content hash.** rev-9 evaluated *classification* globally but still let the per-hash machine act per reference, so a recovery could remove `h.blob` while another generation's reference to `h` stayed **retained** — manufacturing the dangling reference the whole design exists to prevent, and rev-9's own compound ladder said so out loud. Rev-10 removes that composition instead of documenting it: `RecoverPendingPurge` first **claims `h` globally** — one CAS-published index rewrite that sets every reference to `h`, in every generation, to removal-pending, taken only while `h.blob` is present, regular and hash-correct — then re-reads and revalidates that no reference to `h` is anything but pending, and only then removes the blob and tombstones every reference. Where the blob is already absent the claim is published straight to tombstones, which is exactly the shipped dangling repair. **The blob is removed only when the index, at the revalidation immediately before the removal, holds no retained reference to `h`** (§9.3, §9.7.2, §7.10 CP12/CP12a, ADR-035 D10/D16). Ordinary commands and previews never act on an owned hash: ownership outranks every other classification of the same hash, a mutating `prepare` refuses exit 3 `recovery-pending`, and the honest residual — an external index edit inserting a same-hash reference before, between or after the claim — is modelled per window with the CAS residual stated rather than hidden. **X11's storage validation is global and selector-independent**: it observes the whole index before **every** archive mutation, so a mixed tombstone/live-reference hash makes an unrelated `--orphans`, `--blob`, `--generation` or `--all` invocation refuse exit 3 **zero-write**, naming that hash's owning repair; no partial cleanup of unrelated items proceeds around a global inconsistency, and an admitted repair is admitted only for the exact inconsistency it repairs, after global classification (§9.7.1, §9.7.2, §10.5 step 22). **A retained reference beside a present but non-regular or hash-wrong blob is `archive-blob-corrupt`, exit 3**, zero-write for `list`, `doctor` and every ordinary mutation, with one repo-relative route: preserve the unexpected bytes if wanted, remove the managed blob path, then run the confirmed `purge <slug> --blob <h> --yes` that tombstones every now-dangling reference — or restore the exact correct blob and retry — with its cost and the Git-history caveat (§9.3.1, §10.4.1, §10.7). **The abandon table's domain is every argv that requests a *true* abandon**, or that fails parsing/arity/mutex while naming the flag; an explicit `--abandon-transaction=false` selects `generate` and is outside it (§5.2, §6.6). **The one retry heading is a column-0 literal**, alone on its line with no leading or trailing whitespace, in every worked example and every emitter (§9.7.2, §10.2). `list` and `doctor` render every safe observation and exit **3** whenever any exit-3 observation is present alongside an exit-0 one; §6.2 discloses that `--manual`, which writes no archive, is refused zero-write by every X11 global inconsistency as well as by pending state. Matrix: 545 rows; rev-10 amends listed stable rows and adds `PIB-537`…`PIB-545`. |
| rev-11 | Draft — Awaiting Review | Final bounded claim/ordering/route fold of rev-10; no product choice is reopened. **The global claim is total over the hash.** rev-10 gated every removal on *every* reference to `h` being removal-pending while still calling an already-tombstoned same-hash reference a “deterministic no-op”, so an index holding one tombstoned and one retained reference to `h` could never satisfy the gate and the hash stayed owned with no command able to advance it. The exemption is withdrawn, not the gate: the claim CAS sets **every** reference to `h`, in every generation and every wire state, to removal-pending in one rewrite; the absent-blob path publishes **every** reference straight to tombstones with no retained reference surviving; and PIB-545 now derives the claim's reference set as well as the removal's dominator (§9.3, §9.7.1, §9.7.2, §9.3.1, ADR-035 D10/D16). **Pending recovery is the one explicit exception to global validation.** Whole-index X11 runs before every mutation a **new selector** would perform; completing an already-pending purge transaction runs *first*, may finalize its owned hash while an unrelated inconsistency is present, returns terminal `recovered` and never proceeds into the selector — after which the operator's rerun performs the full global scan and may refuse the unrelated hash. Every universal claim is qualified accordingly, the compound ladder of §9.7.2 is reconciled, and a normal mutating `prepare` still refuses `recovery-pending` (§9.3.1, §9.7, §9.7.2, §10.5, new PIB-546). **Corrupt-object removal is type-total.** For an unexpected managed blob object — symlink, directory, FIFO, device node or hash-wrong regular file — the procedures print one exact-path `rm -rf -- <managed blob path>` under an explicit destructive warning, with the `--` terminator and no wildcard, and **withdraw** rev-10's `cp` preservation promise, which was unexecutable on a directory and misleading on a symlink; an operator who wants the object is told to stop and use kind-appropriate tooling. The index-divergence restore route stays separate and names no removal (§9.3.1, §9.7.2, §10.4.1, §10.7, new PIB-547). **Admission is per repair class, fully covered.** rev-10's “sole inconsistency” rule bricked an archive holding two same-class instances; admission now requires one class and a confirmed selection covering every instance of it — `--orphans --yes` clears all unreferenced residues, repeated `--blob` (or an applicable `--all`) clears all same-class hashes — while mixed classes stay a zero-write refusal rendering each route (§9.3.1, §9.7.1, §9.7.2, new PIB-548/PIB-549). **The state map is total and the last residual is pinned**: §9.3's table covers retained/absent and retained/present-unidentifiable across ownership, tombstoned/absent-and-owned and tombstoned/present-unidentifiable, with corrupt dominating mixed and ownership dominating both, so no triple has zero or two routes (new PIB-551); and the external replacement of a managed object between the pre-removal revalidation and the unlink is disclosed as a fifth window and a permanent residual that no string may claim closed (new PIB-550). Matrix: 551 rows; rev-11 amends listed stable rows and adds `PIB-546`…`PIB-551`. |

## Summary

The accepted prerequisite PRD gave `tpatch` a truthful **answer**: which intent
artifacts a feature has, and that their provenance is `unknown`. It
deliberately shipped no way to **act** on that answer, and refused plain
`tpatch prepare <slug>` with a reserved-surface exit 4
(`docs/prds/PRD-artifact-validation-and-provenance.md:356-382`).

This PRD defines that action. It adds three mutating modes and one recovery
escape hatch to the same verb:

```text
tpatch prepare <slug>                        # Path A: generate ONLY the missing required artifacts
tpatch prepare <slug> --manual               # Path B: adopt an already-complete hand-authored bundle
tpatch prepare <slug> --regenerate           # explicit, archived, coherent overwrite of the whole bundle
tpatch prepare <slug> --abandon-transaction  # unblock a slug an interrupted run left divergent (§6.6)
```

Four properties are load-bearing, and each is stated as a limit rather than a
slogan:

1. **Default preserves.** Every canonical artifact that is already
   `present-nonempty` — **including the structured sidecar
   `artifacts/analysis.json`** — is never opened for writing, never staged and
   never renamed over. Only genuinely missing required artifacts are generated,
   and only when the missing set is a **dependency-coherent suffix** of
   analysis → spec → exploration (§6.1.2). An existing required artifact that
   is `present-empty`, `invalid-structured`, `symlink-refused`, `not-regular`,
   `unreadable`, `oversize` or `unstable` is **refused**, not silently
   overwritten (§6.1, §7.2).
2. **`--regenerate` cannot lose bytes, and cannot silently downgrade them.**
   Before the first canonical mutation, every artifact it is about to replace
   is copied into a durable, content-addressed, immutable **intent archive**
   inside the feature directory — after passing the ADR-027 D3 redaction scan,
   which **refuses** rather than archiving secret-shaped content (§9.6). The
   archive is a *byte-recovery* mechanism while its blob remains present and is explicitly **not** a
   provenance representation (§9, ADR-035 D9). `--regenerate` additionally
   requires a **successful configured provider** unless the operator opts in to
   heuristic replacement with `--allow-heuristic` (§11.3): a hand-authored
   bundle is never replaced by boilerplate because a provider call timed out.
   This selection is what fires the architecture gate and requires ADR-035
   (§8.4).
3. **The transaction guarantee is command-boundary, verified at a point this
   command owns.** An ordinary rename sequence over six files cannot make them
   appear simultaneously to a concurrent reader, and this PRD never claims it
   does. What is guaranteed is (a) at a **final verification point under the
   lock, immediately before the command returns**, every publication entry
   matches its intended new image or the command reports divergence, (b) a
   crash inside the publication window is recoverable to all-old, or refused
   with everything preserved, and (c) the exposure window is bounded and
   enumerated (§7.1). External writes that land **after** that verification
   point are outside the postcondition and are stated as such.
4. **No new lifecycle state.** A successful mutation ends at the existing
   `defined` (`internal/store/types.go:11`). `--check` mode is byte-identical
   to the accepted contract. Nothing calls `prepare`; `next`, `cycle` and the
   individual phase commands keep their current routing except for the
   enumerated deltas in §12.6.

The publication unit — the thing that is all-old or all-new at the command
boundary — is exactly WP-005 Agreed item 7's set
(`docs/whitepapers/WP-005-spec-driven-workflows.md:75-81`): `analysis.md`,
`spec.md`, `exploration.md`, the structured sidecar `artifacts/analysis.json`,
the archive index, and the final `status.json` transition. `FEATURES.md` is
**not** in that set: it is a derived index the shipped writer refreshes
best-effort (`internal/store/store.go:363-377`), and §12.3.1 states its
semantics truthfully instead of pretending it does not exist.

### What this PRD does not claim

- It does not claim the generated artifacts are *good*. The accepted
  disclaimer ("Structural presence only. This report does not certify semantic
  quality.", `docs/prds/PRD-artifact-validation-and-provenance.md:1796-1798`)
  applies verbatim to everything this command writes.
- It does not claim durable per-artifact provenance. `prepare --check` keeps
  reporting `provenance: unknown` for every artifact after this PRD ships, for
  every artifact this command writes, and §9.8 states exactly why the archive
  does not change that.
- It does not claim multi-file atomicity (§7.1).
- It does not claim that the archive introduces **no new exposure class**.
  rev-0 said that and it was wrong: §9.6 discloses the durable tracked
  retention the archive creates and the removal route that bounds it.
- It does not claim `prepare` runs no Git command. rev-0 said that and it was
  wrong: §7.13 enumerates the exact read-only Git invocations, when each runs,
  and what happens when Git is absent or unusable.
- It does not claim bounded runtime. ADR-034 D16 withdrew every such claim for
  the read half; the write half adds provider calls, so the honest statement is
  a **per-phase and total deadline** (§11.5), not a guarantee of termination
  without one.
- It does not mandate any spec-driven methodology on downstream users. WP-005
  Agreed item 3 (`docs/whitepapers/WP-005-spec-driven-workflows.md:56-58`) is
  asserted mechanically, not assumed (§14.2, PIB-217).


## 1. Problem statement

### 1.1 There is no way to complete an intent bundle in one step

Today an operator completes a bundle by running up to three separate commands
(`analyze`, `define`, `explore`), each of which writes its artifact and
advances state independently (`internal/workflow/workflow.go:90-105`,
`:151-155`, `:196-200`). If the second fails, the tree keeps the first
artifact and a `analyzed` state — a half-bundle that no command reports as
such. WP-005 Agreed item 7 names this the thing a mutating `prepare` must fix
(`docs/whitepapers/WP-005-spec-driven-workflows.md:75-81`).

### 1.2 `cycle` is not that command

`cycle` runs analyze → define → explore → implement → apply → record in one
process (`internal/cli/phase2.go:26-32`). It calls the same incremental phase
functions in sequence (`internal/cli/phase2.go:62-96`), asserts state after
each (`internal/cli/phase2.go:69-71`), and continues past `defined` into
`implementing` and beyond unless `--skip-execute` is passed
(`internal/cli/phase2.go:122-126`). It is a **pipeline**, not a transaction:
every intermediate write is published to the canonical tree as it happens, and
a failure at step 3 leaves steps 1 and 2 on disk. §4 develops this as the
required existing-primitives pre-flight.

### 1.3 The phase functions cannot be reused as-is

`RunAnalysis` writes `artifacts/analysis.json`
(`internal/workflow/workflow.go:90`), then `analysis.md`
(`internal/workflow/workflow.go:96`), then mutates state
(`internal/workflow/workflow.go:103`) — three canonical mutations inside one
call, in that order, with a live `*store.Store`. `RunDefine` and `RunExplore`
have the same shape (`internal/workflow/workflow.go:151-155`, `:196-200`).
Worse, the retry helper writes raw provider responses into the canonical
artifacts directory *during* generation
(`internal/workflow/retry.go:105-109`), as does `RunAnalysis`'s provider-error
fallback (`internal/workflow/workflow.go:72`, `:80`). A `prepare` that called
these functions would have published four to twelve canonical files before it
knew whether the bundle would complete. WP-005 Turn 3 states exactly this
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117`).

### 1.4 Overwriting is currently unconditional and unrecoverable

`WriteFeatureFile` → `writeFile` → `os.WriteFile`
(`internal/store/store.go:443-449`, `:918-923`) truncates in place. There is no
preimage capture anywhere in the intent path. `tpatch define` run twice
destroys a hand-authored `spec.md` with no recovery route inside `tpatch`, and
`.tpatch/` is frequently untracked in real repositories — the CLI has a
dedicated helper to detect that (`internal/cli/cobra.go:3405-3407`). Any
`--regenerate` flag built on today's primitives would be a byte-shredder.

### 1.5 Path B has no completion verb

The documented Path B flow authors three files by hand and then runs three
`--manual` commands (`docs/path-b-operator-guide.md:61-73`). Each of those
commands validates only that the file *exists*
(`internal/store/manual.go:56-66`) — a zero-byte `spec.md` advances the feature
to `defined`. There is no single "I have authored the whole bundle, adopt it"
step, and no step that checks the bundle as a set before committing the
transition.

## 2. Goals / Non-goals

### 2.1 Goals

1. One command that takes a feature from any allowed pre-`defined` state to a
   **complete** intent bundle at `defined`, or leaves the tree exactly as it
   found it.
2. Preservation by default: never overwrite an existing non-empty canonical
   intent artifact — including the structured sidecar — unless the operator
   explicitly asked for it.
3. A `--regenerate` route whose destructiveness is bounded by a durable,
   in-repository byte archive, so prior hand-authored content is recoverable
   without Git, **and** whose replacement content is never silently downgraded
   to heuristic boilerplate.
4. A truthful transaction contract that distinguishes command-boundary
   all-or-nothing, crash recoverability, and instantaneous multi-file
   visibility — claims only the first two, and scopes the first to a
   verification point this command owns.
5. Crash recovery that decides from **evidence**, never from a recorded phase
   label, refuses rather than guesses when the evidence is divergent, and
   leaves no slug blocked without a named route out: lock ownership dies with
   the process, and every refusal names either the command that clears it or,
   where the environment denies that command, the manual procedure that does
   (§6.6, §9.7.2, §10.4).
6. Behavior compatibility: `--check`, `next`, `cycle`, the individual phase
   commands, `status`, `verify`, `doctor`, `land`, `record` and `reconcile` are
   unchanged except for the enumerated deltas in §12.6.
7. Reuse of the accepted ADR-034 rooted-inspection boundary for every canonical
   read, and of `*os.Root`'s handle-relative write primitives for every
   canonical write, with no path-based write after a rooted check.
8. Determinism in tracked artifacts: no wall-clock field, content-addressed
   identifiers, canonical JSON, stable key order.
9. A **bounded** archive: durable retention is disclosed, an operator can list
   what is retained, purge it safely, and remediate an accidentally archived
   secret without hand-editing tracked JSON.

### 2.2 Non-goals

1. **No new `FeatureState`.** WP-005 Agreed item 6
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:71-74`).
2. **No semantic quality judgement.** No thinness score, no lint, no heading
   requirement, no "stub detector".
3. **No durable per-artifact provenance.** The WP-005 provenance ADR trigger
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2951-2985`) stays
   **unfired**; §9.8 and ADR-035 D9 state why the archive does not fire it.
4. **No change to `--check`'s report schema, exit envelope or bytes.**
5. **No mandatory SDD methodology** for downstream users.
6. **No restore verb.** Recovering an archived blob in v1 is an ordinary file
   copy (§9.5, Q4). Removal, unlike restoration, *is* in scope (§9.7), because
   an unbounded tracked retention surface with no removal route is a privacy
   defect rather than a missing convenience.
7. **No `--all` sweep**, no multi-slug form, no `--fix`.
8. **No interactive confirmation prompt** in the mutating publication modes
   (§5.4). The archive-purge and abandon routes are the two exceptions, and
   they use the shipped non-interactive `--yes` gate, not a prompt.
9. **No implement/apply/record orchestration.** `prepare` stops at `defined`.
10. **No `--force` in any mode.** There is no flag anywhere in this design that
    overwrites a canonical file whose current bytes failed a compare-and-swap,
    and no flag that removes a lock a live process holds.


## 3. Terminology

| Term | Meaning in this document |
|---|---|
| **Intent bundle** | `analysis.md`, `spec.md`, `exploration.md` — the three canonical Markdown artifacts, per the accepted PRD §6.2 (`docs/prds/PRD-artifact-validation-and-provenance.md:432-436`). |
| **Publication set** | The ordered list of canonical files this invocation will create or replace, plus the archive index and `status.json`. Computed at preflight, frozen before the journal is written, and bound into the journal by a digest (§7.2, §7.5). |
| **Publication unit** | The set that is all-old or all-new at the command-owned final verification point (§7.1 T1). Identical to the publication set. Excludes `FEATURES.md` (§12.3.1). |
| **Preimage** | The identity (existence, SHA-256, size, mode) of a canonical or metadata file as captured under the transaction lock immediately before publication. |
| **New-image** | The identity of the staged bytes that will replace it. |
| **Semantic CAS** | A compare-and-swap on **identity, not on a lock**: before every publish, rollback or remove of one entry, the file's current identity is recomputed and compared to the exact identity that step expects (preimage for a publish, new-image for a rollback, absence for a create-undo). A mismatch refuses that step and preserves the bytes (§7.8). |
| **Staging tree** | `.tpatch/local/intent-prepare/<slug>/stage-<12hex>/` — gitignored scratch where generated bytes live before publication. |
| **Journal** | `.tpatch/local/intent-prepare/<slug>/journal.json` — the durable **undo** record, bound to its plan by `plan_digest` (§7.5). |
| **Transaction lock** | One kernel, process-lifetime advisory `flock` on the held workspace-root directory descriptor. It is workspace-wide, leaves no file or durable store, and is released by the OS on holder death (§7.4). |
| **Intent archive** | `.tpatch/features/<slug>/artifacts/intent-archive/` — the tracked, content-addressed, immutable store of replaced bytes (§9). |
| **Generation** | One archive record naming the set of artifacts replaced by a single `--regenerate` publication (§9.3). Content-addressed; not a timestamped event. |
| **Tombstone** | An index entry whose blob was deliberately purged (§9.7): the generation record and immutable `content_sha256` survive, `blob` is empty and `purged` is `true`, so X10 remains computable and the index never claims bytes it no longer has. |
| **Orphan blob** | A blob file written by an attempt that never published an index entry (crash, or a refusal after the blob write). Additive, harmless, reported and removable (§9.7.3). |
| **Coherent suffix** | The rule that the default mode may only generate a suffix of the dependency chain analysis → spec → exploration, so a generated artifact is never published beneath a downstream artifact that did not derive from it (§6.1.2). |
| **Mode** | Exactly one of `check`, `generate` (default), `manual`, `regenerate`, `abandon` (§5.2). |
| **Inspector** | The accepted read-only classifier from the prerequisite PRD §7, reused unchanged. |


## 4. Existing-primitives preflight

WP-005 Turn 3 requires this section by name
(`docs/whitepapers/WP-005-spec-driven-workflows.turns.md:118-123`). The
question is not "is a new command nicer" but "can a shipped primitive already
carry this responsibility".

| Primitive | What it does today | Why it cannot carry this |
|---|---|---|
| `analyze\|define\|explore` (Path A) | One provider call, one artifact write, one state transition each (`internal/workflow/workflow.go:90-105,151-155,196-200`) | Each publishes to the canonical tree before the next runs. Three invocations cannot be all-or-nothing, and a failure in the middle is not observable as a half-bundle. |
| `analyze\|define\|explore --manual` (Path B) | Validates the artifact **exists**, then advances state (`internal/store/manual.go:56-81`) | Presence-only; a zero-byte file advances state. One artifact per invocation. No set-level view. Accepted PRD §12 deliberately leaves these loose (`docs/prds/PRD-artifact-validation-and-provenance.md:2986-3015`), and this PRD does not tighten them (§12.4). |
| `cycle` | Sequential pipeline through `record` (`internal/cli/phase2.go:26-32,62-121`) | Not a transaction (§1.2); overshoots `defined`; regenerates unconditionally, so it is exactly the byte-shredder §1.4 describes. |
| `next` | Emits one action, inferring the `defined` sub-state from `exploration.md` presence (`internal/cli/phase2.go:437-446`) | Advisory only; writes nothing; cannot generate. |
| `prepare --check` (accepted) | Read-only classification and readiness verdict | Writes nothing by contract (`docs/prds/PRD-artifact-validation-and-provenance.md:2840-2866`). It is the **input** to this PRD, reused wholesale. |
| `store.writeFileAtomic` | Path-based temp + `Chmod` + `Write` + `Sync` + `Close` + `Rename` + parent-dir `Sync` (`internal/store/store.go:878-917`) | Correct historical single-file shape, but not safe as a `prepare` writer because its names are path-based. `prepare` uses the rooted equivalent even for `--manual` (§6.2, §7.7.3). |
| `gitutil.DurableWriteFile` | Path-based `os.CreateTemp` in a named directory, then write + fsync + close + rename + directory fsync; exported so the land journal writes its evidence with identical durability (`internal/gitutil/index_snapshot.go:455-500`) | The right **durability shape** for a single control file, and the reason `land`'s journal survives a crash. It is nonetheless path-based: it takes a directory *pathname* and creates its temp with `os.CreateTemp`, so an ancestor swap between check and write is not excluded. `prepare` reimplements the same shape on the rooted primitives for the journal, both raw metadata preimages, staging and every other `.tpatch/local/` write (§7.5, §7.7.1). It is a **shape precedent, never a callee** (PIB-454, PIB-456). |
| `land`'s journal | Durable journal + evidence-based recovery under the index lock (`internal/cli/land_journal.go:445-482`) | The right **shape**, wrong direction: land must roll *forward* because `git commit` already advanced HEAD and cannot be undone (`internal/cli/land_journal.go:11-23`). Nothing in a prepare publication is irreversible, so this PRD's journal is undo-only (§7.5, ADR-035 D5). Its `O_EXCL`-plus-nonce lock is reused **only as a rejected alternative** (§7.4.4). |
| `rescap.AcquireLock` | Kernel `flock(2)` on an open descriptor, `LOCK_EX\|LOCK_NB`, released by descriptor close — including on SIGKILL, crash or power loss (`internal/rescap/lock_unix.go:63-88`) | A useful primitive precedent only. It is a resource-capture **file** lock and is not extracted or reused as prepare's authority. §7.4 instead adds a prepare-owned held-root **directory** helper; `rescap` remains byte-identical. |
| `store.PublishBatch` | Content-addressed immutable batch + atomically rewritten pointer, with a semantic-body compare before any rewrite (`internal/store/resource_publish.go:230-285`, `internal/store/resource_publish.go:358-399`) | The right **shape** for the archive (§9) and the right **precedent** for semantic CAS (§7.8), reused as design precedents. It publishes one pointer, not a six-file set, so it is not the transaction either. |
| `redact.Scan` | The closed six-class secret scan applied to in-memory bytes before anything is persisted (`internal/redact/redact.go:53-72,81-90`) | Reused verbatim as the archive's write precondition (§9.6), because ADR-027 D3 makes redaction a precondition for **every** tpatch-owned writer, not an archive-specific idea. |
| `MkdirAllAndSyncChain` / `SyncDir` / `RandomHex12` | Crash-safe directory and scratch primitives (`internal/store/fsdurable.go:22-33,41-52,96-103`) | Reused verbatim. They are ingredients, not the recipe. |
| `rescap` scratch + local-ignore gate | `.tpatch/local/<...>` scratch with a mandatory gitignore contract (`internal/rescap/scratch.go:47-62`) | Reused for the staging tree, lock and journal location (§7.3, §13.5) — but the gate **runs Git** and refuses when Git is unavailable (`internal/workflow/session_ignore.go:138-145`), so §7.13 makes its application conditional and discloses it instead of claiming this command runs no Git. |
| `session purge` | Dry-run-by-default deletion of local buffers with an explicit `--yes` gate and a scope mutex (`internal/cli/session.go:353-419`) | Reused as the **grammar precedent** for the archive retention surface (§9.7). It purges a gitignored lane; the archive is tracked, so §9.7 adds the tombstone and reference rules that a tracked store needs. |

**Conclusion.** Every ingredient exists; the composition does not. The new
surface is the composition (a transaction over a publication set) plus one new
durable object (the archive) plus one bounded retention surface for it.
Nothing else is invented.


## 5. CLI grammar and mode selection

### 5.1 Authorized grammar (v1, complete)

```text
tpatch prepare <slug> --check       [--json] [--quiet] [--path <dir>]
tpatch prepare <slug>               [--json] [--quiet] [--path <dir>] [--timeout <d>] [--timeout-phase <d>] [--no-retry] [--dry-run] [--allow-heuristic]
tpatch prepare <slug> --manual      [--json] [--quiet] [--path <dir>] [--dry-run]
tpatch prepare <slug> --regenerate  [--json] [--quiet] [--path <dir>] [--timeout <d>] [--timeout-phase <d>] [--no-retry] [--dry-run] [--allow-heuristic]
tpatch prepare <slug> --abandon-transaction [--json] [--quiet] [--path <dir>] [--yes]

tpatch feature intent-archive list  <slug> [--json] [--quiet] [--path <dir>]
tpatch feature intent-archive purge <slug> (--blob <hash>... | --generation <id>... | --orphans | --all) [--yes] [--json] [--quiet] [--path <dir>]
```

- `<slug>` — exactly one, required, validated by the accepted canonical-slug
  grammar before any path is composed
  (`docs/prds/PRD-artifact-validation-and-provenance.md:696-772`). No new slug
  rule is introduced.
- `--check` — the accepted read-only mode. **Unchanged in every respect**
  (§12.1).
- `--manual` — Path B adoption. No provider call, no artifact write (§6.2).
- `--regenerate` — explicit archived overwrite of the whole bundle (§6.3).
- `--abandon-transaction` — the operator escape hatch from a divergent
  interrupted run. Touches **no** canonical file (§6.6).
- `--allow-heuristic` — legal in `generate` and `regenerate` only, and it
  therefore appears on **both** of their grammar lines above. With
  `--regenerate` it opts in to replacing the bundle with heuristic output when
  the provider is missing or fails; without it, `--regenerate` requires a
  successful configured provider (§11.3). In the default `generate` mode it is
  a **legal no-op** that changes nothing and emits advisory
  `allow-heuristic-redundant`, because fallback is already the default there
  (§5.2, §10.3, §11.3.1). rev-6 stated that semantics correctly and still left
  the flag off the default grammar line; rev-7 puts it there without changing
  a word of the semantics. Rejected with `--check`, `--manual` and
  `--abandon-transaction` as a parse-time exit 1.
- `--dry-run` — report the plan; write nothing at all, including no lock, no
  journal and no staging tree; make no provider call (§6.4).
- `--timeout` — the **total** deadline for all generation in the invocation.
  Default `180s` (§11.5, Q2). Rejected in `--check`, `--manual` and
  `--abandon-transaction`, which make no provider call.
- `--timeout-phase` — the per-generator deadline. Default `90s`, and always
  clamped to the remaining total (§11.5). Same mode restrictions as
  `--timeout`.
- `--no-retry` — disables validator-driven retry, exactly as the phase commands
  do today (`internal/cli/cobra.go:630`, `internal/workflow/retry.go:83-85`).
  Rejected in `--check`, `--manual` and `--abandon-transaction`.
- `--yes` — required to make `--abandon-transaction` and
  `feature intent-archive purge` act; without it both are previews. The shipped
  `session purge` gate (`internal/cli/session.go:353-419`) is the precedent.
  On `prepare`, `--yes` in any mode other than `abandon` — **including the
  default mode, which is the absence of a mode flag** — is rejected by the
  command's own preflight with one fixed literal and exit 1 (§5.2, §10.5
  step 1a). `feature intent-archive purge` registers and validates its own
  `--yes` independently; the prepare rule says nothing about it.
- `--json`, `--quiet` — the report routing shipped by the accepted contract
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2367-2419`), extended
  to the mutating report schema of §10.
- `--path` — the inherited root persistent flag (`internal/cli/cobra.go:66`).

No other flag is registered on `prepare`. There is no `--all`, no `--fix`, no
`--force`, no `--restore`, no `--format`, no `--interactive`.

**Exactly twelve flags are registered on `prepare`** — `--check`, `--manual`,
`--regenerate`, `--abandon-transaction`, `--allow-heuristic`, `--dry-run`,
`--yes`, `--json`, `--quiet`, `--timeout`, `--timeout-phase`, `--no-retry` —
plus the inherited persistent `--path`. Every one of the twelve appears on at
least one grammar line above, in every mode where it is legal, and on no line
where it is not. PIB-020 asserts the registration set and PIB-520 asserts that
the grammar block and that set agree, so a legal-but-unlisted flag is a
mechanical failure rather than a reading exercise.

**Why the retention surface is a `feature` subcommand and not a sixth prepare
mode.** `tpatch feature` already groups per-feature management verbs
(`internal/cli/feature_deps.go:43-53`), including `feature resource remove` and
`feature resource clear` (`internal/cli/feature_resource.go:655,699`). A purge
is management of a stored artifact, not preparation of intent; putting it on
`prepare` would make the mode mutex non-total and would give a *generation*
verb a *deletion* mode. §9.7 states the full contract.

### 5.2 Mode selection is total and mutually exclusive

`--check`, `--manual`, `--regenerate` and `--abandon-transaction` are declared
mutually exclusive with cobra's `MarkFlagsMutuallyExclusive`, so any two of them
is a **parse-time** error and exits `1` before `RunE` runs. Exactly one mode is
selected.

All four are boolean flags, so "set" in the table below means **present with a
true value** — the bare `--flag` spelling or an explicit `--flag=true`. An
explicit `--flag=false` is a *false* value and selects no mode; a lone
`prepare <slug> --abandon-transaction=false` is therefore an ordinary
`generate` invocation and is given its own row so the reading cannot drift.
Cobra's mutex, by contrast, is keyed on flag **presence** (`Changed`), not on
value, so `--check --abandon-transaction=false` is still a parse-time exit `1`
even though neither flag selects `abandon`. Both facts matter to §6.6's gate
table, whose domain is every argv that requests a *true* abandon or fails
parsing while naming the flag (§6.6, PIB-511, PIB-537).

| `--check` | `--manual` | `--regenerate` | `--abandon-transaction` | Mode |
|---|---|---|---|---|
| set | — | — | — | `check` |
| — | set | — | — | `manual` |
| — | — | set | — | `regenerate` |
| — | — | — | set | `abandon` |
| — | — | — | explicitly `=false` | `generate` (default) — the flag was named but no mode was requested |
| — | — | — | — | `generate` (default) |

`--timeout`, `--timeout-phase` and `--no-retry` are additionally declared
mutually exclusive with `--check`, `--manual` and `--abandon-transaction`.
`--allow-heuristic` is declared mutually exclusive with `--check`, `--manual`
and `--abandon-transaction`, so it is legal only in `generate` and
`regenerate` mode — and in `generate` mode it is a no-op that emits advisory
`allow-heuristic-redundant`, because fallback is already the default there
(§11.3). `--dry-run` is mutually exclusive with `--check` **and with
`--abandon-transaction`**, which is preview-by-default already (§6.6) and
would otherwise have two spellings for one behavior; so `--dry-run` is legal in
exactly the three modes §6.4 describes.

**`--yes` is validated by this command, not by a cobra mutex.** `--check`,
`--manual` and `--regenerate` are flags, so cobra can exclude `--yes` from
them; the default `generate` mode is the **absence** of every mode flag, and
`MarkFlagsMutuallyExclusive` cannot express a mutex against an absence. rev-5
declared one anyway, which is not implementable. So `--yes` carries **no**
cobra mutex at all: after parsing, `prepare`'s own `RunE` preflight computes the
selected mode from the table above and, if `--yes` is set and the mode is not
`abandon`, prints exactly this literal to stderr and exits `1` having written
nothing:

```text
prepare: --yes is only valid with --abandon-transaction
```

One code path and one literal therefore cover `--yes --check`,
`--yes --manual`, `--yes --regenerate` and bare `--yes`, so the message never
depends on which illegal spelling the operator used. It runs at §10.5 step 1a —
after pflag parsing and **before** the `--check` handoff — so no mode's
contract, including the accepted read-only one, ever observes a stray `--yes`.
Bare `--abandon-transaction` remains a legal preview and
`--abandon-transaction --yes` remains the acting form (§6.6). The
`feature intent-archive purge` surface owns its own `--yes` registration and
validation and is untouched by this rule (PIB-243, PIB-502, PIB-503).

### 5.3 Behavior deltas this grammar creates — enumerated, not silent

The accepted contract reserved plain `prepare` and both mutating flags. Shipping
this PRD changes observable behavior on the `prepare` surface, and the accepted
PRD anticipated exactly this
(`docs/prds/PRD-artifact-validation-and-provenance.md:396-403`). The table is
**complete over every flag this PRD registers**: before this PRD, the accepted
implementation registers `--check`, `--json`, `--quiet` only, so every other
flag below is today a cobra `unknown flag` → exit 1.

| Input | Before this PRD | After this PRD |
|---|---|---|
| `tpatch prepare <slug>` | exit `4`, frozen refusal line, no report (`docs/prds/PRD-artifact-validation-and-provenance.md:356-382`) | a real Path A run (§6.1) |
| `tpatch prepare <slug> --manual` | exit `1`, `unknown flag: --manual` | Path B adoption (§6.2) |
| `tpatch prepare <slug> --regenerate` | exit `1`, `unknown flag: --regenerate` | archived coherent overwrite (§6.3) |
| `tpatch prepare <slug> --abandon-transaction` | exit `1`, `unknown flag` | preview of the abandon plan; with `--yes`, performs it (§6.6) |
| `tpatch prepare <slug> --allow-heuristic` | exit `1`, `unknown flag` | legal in `generate` (a no-op that emits advisory `allow-heuristic-redundant`) and with `--regenerate`; exit `1` cobra mutual-exclusion text with `--check`, `--manual` or `--abandon-transaction` |
| `tpatch prepare <slug> --dry-run` | exit `1`, `unknown flag` | plan-only report (§6.4) |
| `tpatch prepare <slug> --timeout 5s` | exit `1`, `unknown flag` | total generation deadline (§11.5) |
| `tpatch prepare <slug> --timeout-phase 5s` | exit `1`, `unknown flag` | per-generator deadline (§11.5) |
| `tpatch prepare <slug> --no-retry` | exit `1`, `unknown flag` | single-attempt generation (§11.4) |
| `tpatch prepare <slug> --yes` | exit `1`, `unknown flag` | exit `1`, the fixed literal `prepare: --yes is only valid with --abandon-transaction` from this command's own preflight (§5.2, §10.5 step 1a) — **not** cobra mutual-exclusion text, and identical for `--yes` beside `--check`, `--manual` or `--regenerate` |
| `tpatch prepare <slug> --check --manual` | exit `1`, `unknown flag` | exit `1`, cobra mutually-exclusive error |
| `tpatch prepare <slug> --check --regenerate` | exit `1`, `unknown flag` | exit `1`, cobra mutually-exclusive error |
| `tpatch prepare <slug> --check --dry-run` | exit `1`, `unknown flag` | exit `1`, cobra mutually-exclusive error |
| `tpatch feature intent-archive …` | exit `1`, `unknown command` | the retention surface (§9.7) |

Consequences that must be carried through, each with an acceptance row:

1. Exit `4`'s reserved-surface population **disappears** and the frozen refusal
   string is deleted. Exit `4` is left **retired with no population** rather
   than rebound to a new meaning, so a harness that hard-coded "4 means
   `prepare` needs `--check`" can never silently misread a different condition
   as that one (§10.4, PIB-013, PIB-014).
2. The accepted PRD's `--help` mitigation for the `apply --mode prepare` name
   collision (`docs/prds/PRD-artifact-validation-and-provenance.md:344-355`)
   stays mandatory and must be updated to describe five modes, not one
   (PIB-016, PIB-017).
3. Every `--check`-plus-mutating-flag row still exits `1` and still writes
   nothing, so the *observable class* is unchanged even though the message text
   changes (PIB-011, PIB-012, PIB-235).
4. Each newly registered flag has its own delta row (PIB-236 … PIB-243), so
   "the flag delta table is complete" is a mechanical assertion, not a claim
   (PIB-244).

### 5.4 No confirmation prompt — decision and justification

The three publication modes perform no interactive confirmation, and there is
no `--yes` on them.

- The repository's only interactive precedent is `cycle --interactive`
  (`internal/cli/phase2.go:50`), which is opt-in and off by default.
- A prompt is not a safety mechanism here: the archive (§9) is. A prompt would
  be theater on top of a mechanism that already makes the operation
  recoverable.
- Prompts break harnesses and non-TTY invocation, and this command is
  explicitly harness-facing.
- The preview route is `--dry-run`, which is deterministic and scriptable.

`--regenerate` is itself the explicit act of consent: it is a flag the operator
must type, it is never implied by any other flag, and it is never the default.

**The two deletion routes are different and are gated differently.**
`--abandon-transaction` and `feature intent-archive purge` destroy state that
no archive backs up, so both are **previews by default** and both require the
non-interactive `--yes` flag to act — the shipped `session purge` shape
(`internal/cli/session.go:353-419`). That is a flag, not a prompt, so it stays
harness-compatible.


## 6. Mode contracts

### 6.1 `tpatch prepare <slug>` — Path A, missing-only

**Intent**: complete the bundle without touching anything that already exists.

**Step 1 — inspect.** Run the accepted inspector over the four artifacts and
`status.json` (`docs/prds/PRD-artifact-validation-and-provenance.md:1653-1782`),
through one held `*os.Root` per ADR-034 D1. No second path model.

**Step 1a — the generation input.** All three shipped generators read
`request.md` and fail hard when it cannot be read
(`internal/workflow/workflow.go:37-40,112,160`). `prepare` therefore captures
`request.md` through the same bounded rooted read and **refuses before any
generation** when it is absent, empty, unsafe or unreadable: exit 3,
`request-unreadable`, zero mutation, no provider call (PIB-233). `--manual`
generates nothing and therefore does **not** require it (PIB-234).

#### 6.1.1 Per-artifact admissibility

For each of the three **required** Markdown artifacts, the inspector state
decides:

| Inspector state | Default-mode disposition |
|---|---|
| `present-nonempty` | **Preserve.** Not staged, not opened for writing, not renamed over. Its bytes are read only as generation context (§11.2). |
| `absent` | **Candidate to generate.** Enters the publication set as a `create` entry **iff** §6.1.2's coherence rule admits it. |
| `present-empty` | **Refuse** (exit 2, `artifact-empty-not-overwritten`). An empty file is *present*; overwriting it is the `--regenerate` route. |
| `invalid-structured` | Not reachable for Markdown (sidecar only). |
| `symlink-refused` | **Refuse** (exit 3, `artifact-unsafe`). Never followed, target never named. |
| `not-regular` | **Refuse** (exit 3, `artifact-unsafe`). |
| `unreadable` | **Refuse** (exit 3, `artifact-unsafe`). |
| `oversize` | **Refuse** (exit 3, `artifact-unsafe`). |
| `unstable` | **Refuse** (exit 3, `artifact-unstable`). |

The table is **total** over the accepted nine-value enum
(`docs/prds/PRD-artifact-validation-and-provenance.md:1765-1773`), and PIB-224
asserts totality mechanically.

The sidecar `artifacts/analysis.json` never causes a *readiness* refusal in the
accepted `--check` sense, exactly as in the accepted contract
(`docs/prds/PRD-artifact-validation-and-provenance.md:437-441`). It does,
however, participate in coherence, because a sidecar is derived data whose
parent this command may be about to create (§6.1.3).

#### 6.1.2 The coherent-suffix rule — the only admissible missing sets

`spec.md` is generated from `analysis.md` (`internal/workflow/workflow.go:120-129`)
and `exploration.md` from both (`internal/workflow/workflow.go:165-172`). A
default run that generated `analysis.md` **underneath** an existing `spec.md`
would publish an analysis that the spec provably did not derive from, and
nothing on disk would record the mismatch. rev-0 permitted exactly that.

The default mode therefore generates only a **suffix** of the dependency chain
(ADR-035 D19).
The table is total over the eight present/absent combinations of the three
required artifacts (unsafe and empty states have already refused above):

| analysis | spec | exploration | Default-mode outcome |
|---|---|---|---|
| absent | absent | absent | generate all three (plus the sidecar, §6.1.3) |
| present | absent | absent | generate `spec.md`, `exploration.md` |
| present | present | absent | generate `exploration.md` |
| present | present | present | **no-op success** (§6.1.4) |
| absent | present | absent | **refuse**, exit 2, `incoherent-bundle-gap` |
| absent | absent | present | **refuse**, exit 2, `incoherent-bundle-gap` |
| absent | present | present | **refuse**, exit 2, `incoherent-bundle-gap` |
| present | absent | present | **refuse**, exit 2, `incoherent-bundle-gap` |

`incoherent-bundle-gap`'s remediation is self-contained and names both real
routes: `tpatch prepare <slug> --regenerate` (coherent whole-bundle
replacement, prior bytes archived), or author the missing file by hand and run
`tpatch prepare <slug> --manual`. It never suggests deleting a file. PIB-245 …
PIB-250 cover the four refusing rows, the message, and the totality guard.

#### 6.1.3 The sidecar rule — preservation is absolute

1. **A present sidecar is never written by the default mode.** Not when it is
   `present-nonempty`, not when it is `present-empty`, not when it is
   `invalid-structured`. `--regenerate` is the only route that replaces it
   (PIB-251, PIB-252).
2. **The sidecar is generated iff `analysis.md` is generated this run and the
   sidecar is `absent`.** Generating an analysis leaves the tree with a sidecar
   that derives from exactly the analysis being published.
3. **If `analysis.md` is absent but the sidecar is present in any state, the
   run refuses** with `incoherent-bundle-gap` (exit 2 for
   `present-nonempty`/`present-empty`/`invalid-structured`; exit 3 for the
   unsafe states, which refuse earlier as `artifact-unsafe`). A sidecar whose
   parent analysis does not exist describes an analysis this run is about to
   invent, and silently overwriting it is exactly the rev-0 defect (PIB-253,
   PIB-254).
4. **If `analysis.md` is preserved, the sidecar is left exactly as found —
   including left absent.** Synthesizing a sidecar for a preserved (possibly
   hand-authored) `analysis.md` would fabricate structured data that does not
   derive from the preserved bytes, and its presence would then be a false
   Path A signal, which
   `docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920` forbids
   (advisory `analysis-preserved-sidecar-untouched`).

**Consequence for the archive: the default mode has no `replace` entry, ever.**
Every entry it publishes is a `create` over an `absent` path. It therefore
writes no blob, appends no generation, and never creates
`artifacts/intent-archive/` (PIB-255, PIB-256). The archive semantics of every
mode are stated once, unambiguously, in §9.2.1.

#### 6.1.4 Nothing to do

If all three required artifacts are `present-nonempty` and the feature is
already `defined`, the command is a **no-op success**: exit 0,
`action: "none"`, zero bytes written anywhere, including `status.json`
(PIB-035). If they are all present but the state is below `defined`, the
command publishes **only** the `status.json` transition (§6.2's single-file
path) and reports `action: "adopt"` (PIB-036).

**Step 5 — generate, stage, validate, publish.** §11 then §7.

**Step 6 — final state.** `defined`, `last_command: "prepare"`, notes per
§12.3.

### 6.2 `tpatch prepare <slug> --manual` — Path B adoption

**Intent**: "I authored the bundle by hand; adopt it."

- **No provider is constructed, no network call is made, and no artifact byte is
  written.** PIB-045 asserts a provider spy records zero calls; PIB-046 asserts
  all four artifact files are byte-identical afterwards.
- The accepted inspector runs unchanged. Adoption proceeds **only** when the
  verdict is the accepted `ready` — all three required Markdown artifacts
  `present-nonempty` (`docs/prds/PRD-artifact-validation-and-provenance.md:432-436`).
- Not ready → refuse with the accepted `not_ready` semantics, **exit 2**, the
  full per-artifact report, and zero mutation (PIB-047, PIB-048).
- Ready → commit the transition: `state = defined`, `last_command = "prepare"`,
  notes per §12.3.

**One-file publication, rooted and CAS-gated.** `status.json` is the only
publication-set file. The command holds the workspace-wide directory authority,
captures its full
rooted preimage while forming the new status bytes, writes a same-directory
`0600` temporary file through the held `*os.Root`, fsyncs and closes it, then
**immediately** re-captures `status.json` through that root. Only an exact
identity match with the preimage permits `(*os.Root).Rename(temp, status)` and
parent-directory fsync. A changed, appeared, disappeared, unsafe, or unreadable
status target refuses `status-changed` (exit 5) before the rename and before a
`FEATURES.md` refresh; the concurrent bytes survive. This helper preserves the
prior status permission bits, never calls `writeFileAtomic`/`SaveFeatureStatus`,
and is the D2 write primitive for this mode (PIB-399, PIB-400).

One rooted rename is atomic in the ordinary single-file sense. Consequently the
mode still has **no journal and no archive**: it has one publication-set entry,
not a multi-file rollback problem (ADR-035 D3). A crash leaves that entry old or
new, never torn. The CAS is nevertheless required because single-file atomicity
does not authorize clobbering a concurrent status edit.

**Writing no archive does not exempt this mode from archive state, and that is
disclosed here rather than left to be discovered.** `--manual` writes no blob
and no index entry, so it is tempting to read it as unaffected by the archive.
It is not: it is a normal mutating `prepare`, so §7.8 step 5 and the archive
integrity ladder of §10.5 step 22 apply unchanged, and **both** refusal
populations below are zero-write in the strongest sense — no provider
construction, no `status.json` rename, no `FEATURES.md` refresh and a
whole-tree byte snapshot identical to the pre-run tree (PIB-525, PIB-538).

1. **Removal-pending references (§9.3).** If the archive index holds one or
   more removal-pending references for the slug, this mode **refuses** — exit
   3, `recovery-pending` — and it does **not** finalize the pending hash,
   because `RecoverPendingPurge` has exactly one owner and it is
   `feature intent-archive purge --yes` (§7.11). A pending reference makes the
   purge transaction the global owner of its content hash (§9.3), so this
   refusal fires on *any* pending reference in the slug's index, not only on
   one whose hash this mode would have touched — this mode touches none. The
   refusal names that owner: the sanitized
   `tpatch feature intent-archive purge <slug> --all --yes` (or
   `--blob <h> --yes` where one hash covers the whole pending set) with
   `retry_cwd: "workspace-root"` and no inherited `--path`.
2. **Every X11 global storage inconsistency (§9.3.1).** A dangling retained
   reference, a tombstoned reference whose named blob is present — in either
   the globally unreferenced or the still-live subcase — and a retained
   reference beside a present but non-regular or hash-wrong blob **all** refuse
   this mode too, at exit 3 with the code and the one owning repair §9.3.1
   assigns to that observation: `archive-blob-dangling` with the confirmed
   `--blob <h> --yes`; `archive-index-storage-inconsistent` with
   `--orphans --yes` when the hash is unreferenced and with the confirmed
   `--blob <h> --yes` when it is still live; `archive-blob-corrupt` with the
   type-total removal-then-confirmed-purge or restore route. X11's observation is
   **global** — it scans the whole index, not a selection (§9.7.1) — so an
   inconsistency on a hash unrelated to anything `--manual` does still stops
   it. That is deliberate: this mode publishes a lifecycle transition the
   archive's own truthfulness is part of, and proceeding past an index that is
   lying about stored bytes would make the transition a statement made on top
   of state tpatch has already refused to trust.

Running the named owning repair clears the state, after which an ordinary
`--manual` proceeds. The cost is one extra invocation in a rare state; the
alternative — letting a mode that writes no archive finalize an archive
transaction it does not own, or publish past an inconsistency it can see — is
the two-owner defect §7.8's "Why step 5 refuses" rejects (§7.8 step 5,
§9.3.1, §10.4.1, PIB-525, PIB-526, PIB-538).

**The publication set is not the process's whole write footprint.** After a
successful status rename, `prepare` invokes the existing derived-index behavior
through a rooted best-effort `FEATURES.md` helper. It is not journaled, CASed or
covered by T1; §12.3.1 keeps that carve-out. The `--manual` write set is thus
`{status.json} ∪ {FEATURES.md, best-effort}`, but no path-based writer is an
exception to D2.

### 6.3 `tpatch prepare <slug> --regenerate` — archived coherent overwrite

**Intent**: "Throw away the current bundle and produce a coherent new one."

**Scope decision: all three Markdown files plus the sidecar, always.**

Partial regeneration is not offered, and the reason is coherence, not
convenience: `spec.md` is generated from `analysis.md`
(`internal/workflow/workflow.go:120-129`) and `exploration.md` from both
(`internal/workflow/workflow.go:165-172`). Regenerating `spec.md` against an
old `analysis.md` and calling the result a bundle would produce a set whose
parts describe different intents, and nothing on disk would record the
mismatch. Q1 records a future `--only <ids>` as an additive, enumerated
extension.

**Generation authority.** `--regenerate` **requires a configured provider and a
successful provider result for every artifact**, unless `--allow-heuristic` is
passed. Without a provider it refuses **before any mutation** (exit 3,
`provider-required-for-regenerate`); on a provider failure or deadline expiry it
refuses **before publication** (exit 5, `regenerate-generation-failed`), with
zero canonical mutations and zero blobs written. §11.3 states the full policy
and why silent heuristic replacement of a hand-authored bundle is the one
fallback this design refuses to make.

**Preservation.** Before the first canonical mutation, every artifact whose
state is `present-nonempty`, `present-empty` or `invalid-structured` is scanned
under the ADR-027 D3 redaction contract (§9.6) and then copied into the intent
archive (§9). Artifacts that are `absent` archive nothing. Artifacts that are
`symlink-refused`, `not-regular`, `unreadable`, `oversize` or `unstable`
**refuse the whole invocation** (exit 3) — `--regenerate` will not overwrite
something it could not first safely read and archive. A redaction match also
refuses the whole invocation (exit 3,
`archive-content-refused-sensitive`), because exact-byte recovery and
scrubbing are mutually exclusive and archiving a secret silently is the
outcome ADR-027 D3 exists to prevent. This is the single most important
refusal in the document: an overwrite route whose preservation step can
silently fail — or can silently retain a credential forever — is not a
preservation guarantee (PIB-060 … PIB-066, PIB-262 … PIB-267).

**Lifecycle.** Identical to §6.1's step 6.

### 6.4 `--dry-run` — plan, not outcome

`--dry-run` is available in `generate`, `manual` and `regenerate` modes.

**Exact scope.** Dry-run performs **bounded read inspection plus the
non-mutating checks needed to compute a plan**, and nothing else. Concretely,
inside its own branch (§10.5 step 7) it evaluates, in order: the canonical slug
grammar, workspace discovery, the read-boundary platform allowlist, the feature
directory and `status.json` population, the accepted per-artifact inspection,
the pending-journal marker, the mode/flag grammar of §5.2, the lifecycle-state
gate of §12.2, per-artifact admissibility (§6.1.1, §6.3), bundle coherence
(§6.1.2, §6.1.3), the `request.md` capture (§6.1 step 1a) in the generating
modes, the strict archive-index decode and storage observation (X1–X11) where
the plan reads the index, and — for `--regenerate` — whether a provider is
**configured** (§11.3.2). It then prints the plan or the refusal those checks
produced and returns.

- It writes **nothing**: no directory lock acquisition, journal, staging tree,
  directory, or `status.json`. PIB-072 asserts a filesystem spy records zero
  create/write/rename/mkdir calls of any kind.
- It makes **no provider call** (PIB-073). The `--regenerate` provider check is
  a *configuration* check, never a request.
- It runs **no mutating platform or filesystem authority gate, no G1–G4 Git
  process, no local-lane gate, no `flock`, no generation and no write**
  (PIB-079, PIB-080, PIB-440, PIB-461).
- It reads the pending journal **marker** and refuses when recovery is pending,
  but it does not recover, does not strict-decode the journal and does not move
  it.
- It exits `0` when the plan is admissible, and with the same refusal code the
  real run would produce **for the classes it evaluates** (PIB-074 … PIB-076).

**What dry-run reproduces, and what it deliberately does not evaluate.** rev-4
claimed dry-run "produces the same refusal code the real run would" without
qualification, while simultaneously skipping every mutating gate. Both cannot
be true, so the universal claim is withdrawn. The two populations are closed
and total over the §10.4.1 refusal catalog (PIB-464):

| Reproduced by `--dry-run` (evaluated) | Not evaluated by `--dry-run` (the real run may still refuse) |
|---|---|
| `slug-unsafe`, `workspace-not-initialized`, `workspace-unsupported-platform`, `feature-not-found`, `status-malformed`, `status-unreadable`, `request-unreadable`, `artifact-unsafe`, `artifact-unstable`, `state-refused` | `prepare-unsupported-platform`, `lock-filesystem-unsupported`, `directory-flock-unavailable`, `transaction-in-progress` |
| `incoherent-bundle-gap`, `artifact-empty-not-overwritten`, `not-ready` | `local-lane-not-ignored`, `local-lane-unverifiable` |
| `provider-required-for-regenerate` (configuration only) | `regenerate-generation-failed`, `staged-output-invalid`, `archive-content-refused-sensitive` |
| `archive-index-corrupt`, `archive-index-version-unsupported`, `archive-index-foreign`, `archive-index-path-escape`, `archive-index-generation-mismatch`, `archive-blob-dangling`, `archive-index-storage-inconsistent`, `archive-blob-corrupt` | `entry-appeared`, `entry-changed`, `status-changed`, `archive-index-changed`, `workspace-root-changed`, `archive-purge-index-changed`, `archive-blob-shared`, `archive-generation-id-collision`, `archive-purge-partial` |
| `recovery-pending` (its pending-**journal** population is evaluated here; the same code also fires for archive purge, and for a normal mutating `prepare` on a removal-pending index, neither of which a dry-run reaches) | `undo-cas-mismatch`, `recovery-divergent`, `journal-corrupt`, `journal-version-mismatch`, `journal-foreign`, `journal-path-escape`, `journal-forged`, `post-publication-divergence`, `workspace-root-replaced-after-publication`, `archive-purge-evidence-divergent`, `no-pending-transaction`, `abandon-evidence-unsafe` |

The redaction scan is deliberately in the right-hand column. It is defined as a
write precondition immediately before the first blob write (§9.6.1, ADR-035
D15); running it in a plan would create a second scan site and a report that
names matched classes when no write is about to occur.

**The report says which half it evaluated.** Every dry-run report — human and
JSON — carries the closed field `execution_preflight: "not_evaluated"` (one
value in v1) and this verbatim sentence:

```text
Plan only. Generation was not attempted and may still fail. Execution
preflight was not evaluated: the actual mutation can still refuse on
platform, filesystem, Git, lock or recovery grounds.
```

It also carries `dry_run: true` and the planned `actions` array. Because it
cannot run the generators, it is a statement about admissibility, never about
outcome (PIB-077, PIB-462).

**Consequence on an unsupported execution environment.** On Windows, on a
denied root filesystem, and where G1 cannot classify Git, `--dry-run` still
computes and prints a plan (or the refusal its evaluated checks produced) — it
never emits `prepare-unsupported-platform`, `lock-filesystem-unsupported` or
`local-lane-unverifiable`, because it never runs those gates. The
`execution_preflight` field and the sentence above are what stop that plan from
being read as a promise that the real run would proceed (PIB-463).

**Pending recovery makes the plan unknowable, so `--dry-run` refuses.** A
mutating run performs recovery first (§7.11), and recovery can undo published
entries, which changes the artifact states the plan is computed from. rev-0 let
`--dry-run` skip recovery and still print a plan — a plan for a tree that would
no longer exist by the time the real run computed one. rev-1 refuses instead:

- If a journal exists for the slug, `--dry-run` exits `3` with refusal code
  `recovery-pending`, writes nothing, does not take the lock, and does not
  touch or strict-decode the journal (PIB-079, PIB-268, PIB-269).
- The remediation names the two real routes: run the mutating command, which
  recovers under the lock and then **stops** at exit 0 `recovered` so the
  operator can re-run it for the real work (§7.11), or
  `tpatch prepare <slug> --abandon-transaction --yes` when the operator wants
  the slug unblocked without any canonical change.
- Simulating the post-recovery plan was evaluated and rejected: it would
  require *deciding* recovery's outcome without holding the lock and without
  performing it, which is exactly the class of guess §7.11 refuses to make
  (§21).

### 6.5 `--check` — unchanged

See §12.1. The accepted report bytes, schema, exit codes and zero-mutation
contract are preserved exactly, and PIB-198 … PIB-205 are the regressions that
prove it.

### 6.6 `--abandon-transaction` — the safe way out of exit 6

**Intent**: "An interrupted or divergent run has pinned this slug; unblock it
without touching my files."

Exit 6 must never be a permanent state **without a named, applicable route out**
(ADR-035 D13), and §10.4 states which route belongs to which population. rev-0's
only escape was hand-deleting a
gitignored journal, which is an undocumented, unguarded operation an operator
would perform under stress. This mode makes it explicit, previewable and
canonical-safe for the nine journal and publication codes; archive divergence
has its own procedure in §9.7.2, because this mode cannot touch the archive.

**Contract.**

1. **Requires the mutating environment, and only that.** It passes the same
   mutating platform gate and root-filesystem classification as every other
   mutating mode (§7.4.2) and takes the same one workspace-root directory lock
   (§7.4). A live sibling `prepare` therefore refuses it — exit 3,
   `transaction-in-progress` — instead of pulling control state out from under
   a running process.
2. **Bypasses the local-lane Git privacy gate.** G1–G4 do not run in this mode,
   and neither `local-lane-not-ignored` nor `local-lane-unverifiable` is
   reachable from it (§7.13, PIB-492, PIB-493). The gate exists to prove that
   the gitignored lane is really ignored **before tpatch writes new content
   into it**; abandon writes no new content. It renames existing files and
   directories from `.tpatch/local/intent-prepare/<slug>/` into
   `.tpatch/local/intent-prepare/<slug>/abandoned-<12hex>/` — the same lane,
   the same bytes, the same exposure class, one directory deeper. Requiring a
   working `git` to move bytes that are already on disk would make a broken,
   absent or misconfigured Git a **permanent** block on the one route out of
   exit 6, which is exactly the failure ADR-035 D13 forbids. The privacy
   boundary is therefore unchanged and the escape stays reachable.
3. **Branches before any automatic recovery, and before any feature-directory
   read.** Under the lock, the abandon branch is taken **before** journal
   recovery (§7.8 step 4, §7.11) and before the pending archive-hash gate
   (§7.8 step 5), and neither is entered in this mode — so abandon neither
   recovers a journal nor refuses `recovery-pending` on a removal-pending
   index. This ordering is the whole point of the
   mode: a route whose purpose is to move the evidence of an interrupted
   transaction must not first run the automatic recovery that consumes, undoes
   or deletes that evidence, and must not first hit the exit-6 refusal it
   exists to escape. rev-4 placed recovery ahead of the branch, which made the
   escape unreachable for exactly the populations that need it (§7.8, §10.5,
   PIB-452). The branch is likewise taken **before** the feature-directory and
   `status.json` population step (§10.5 step 6), which is **skipped entirely**
   in this mode: abandon's subject is
   `.tpatch/local/intent-prepare/<slug>/`, not `.tpatch/features/<slug>/`, so
   `feature-not-found`, `status-malformed` and `status-unreadable` are
   unreachable from it and an absent, malformed or unreadable feature can never
   block the inspection or the move of local evidence (PIB-509, PIB-510,
   PIB-513).
4. **Validates its own evidence, and does not strict-decode it.** For each
   candidate it checks only what a safe move requires: the name is a rooted,
   root-relative, contained entry in *this* slug's `.tpatch/local/` lane; the
   journal and both preimage files are regular files; each `stage-*` entry is a
   directory; and the destination `abandoned-<12hex>` does not already exist.
   It never applies the J1–J10 binds (§7.5) and never parses journal content,
   precisely so a **corrupt, forged, version-mismatched or foreign** journal is
   abandonable rather than permanently pinning the slug (PIB-449). Evidence
   that fails the kind/containment check refuses exit 3,
   `abandon-evidence-unsafe`, without moving anything.
5. **Reaches success on every abandonable evidence shape.** A corrupt journal
   (any J-bind failure), a divergent tree (CP9), and a perfectly recoverable
   journal (CP4) all abandon successfully — and so does every one of those with
   `git` absent from `PATH`, with `git` unusable, or with the lane not ignored
   (rule 2). In the recoverable case the entries are **not** undone and the
   journal is **not** consumed: it is moved with everything else, and the report
   says so rather than implying a repair (PIB-449, PIB-450, PIB-451, PIB-492,
   PIB-493).
6. **Refuses when there is nothing to abandon.** Exit 3,
   `no-pending-transaction`, when no journal, no preimage file and no
   `stage-*` tree exist for the slug.
7. **A second abandon over prior `abandoned-*` residue is also
   `no-pending-transaction`, and preserves it.** Once rule 9 has moved the
   evidence, the only thing left in the lane is one or more
   `abandoned-<12hex>/` directories. Those are **not** pending-transaction
   evidence: nothing is mid-flight, and re-moving them would nest evidence
   inside evidence and lose the operator's ability to name what they kept. A
   second `--abandon-transaction` therefore exits 3 `no-pending-transaction`,
   moves, rewrites and removes nothing, and its report lists every
   `abandoned-<12hex>/` directory it found by **repo-relative** path together
   with the optional, operator-chosen `rm -rf` form (rule 9). The human line is:

   ```text
   prepare <slug>: abandon refused no-pending-transaction
     No pending transaction: no journal, preimage or staging evidence exists.
     Previously abandoned evidence is preserved and was not touched:
       .tpatch/local/intent-prepare/<slug>/abandoned-<12hex>/
     Nothing under it is tracked. Remove it when you no longer need it:
       rm -rf .tpatch/local/intent-prepare/<slug>/abandoned-<12hex>/
   ```

   The JSON refusal carries `code: "no-pending-transaction"`, the same
   remediation text and an `abandoned` object whose `existing[]` array holds
   those repo-relative directory paths and nothing else (PIB-499).
8. **A pending archive purge is not abandonable, and abandon says so.** If the
   only pending state is a removal-pending archive index (§9.3) with no
   journal, abandon does **not** consume it, does not rewrite the index and does
   not remove a blob — the index is tracked canonical-adjacent state, which
   rule 10 forbids this mode from touching. A removal-pending reference also
   makes the purge transaction the **global owner** of that content hash
   (§9.3), so there is nothing partial for this mode to pick up even in
   principle: the whole hash belongs to one transaction with one command that
   can advance it. It refuses `no-pending-transaction`
   and its remediation names the **one** command that owns that state:
   `tpatch feature intent-archive purge <slug> --blob <hash> --yes` (or
   `--all --yes` where more than one hash is pending), which resumes the
   per-hash state machine. It does **not** offer a mutating `prepare` as a
   second route: since rev-8 a normal mutating `prepare` never runs the
   pending-hash recovery either — it refuses exit 3 `recovery-pending` and
   names this same purge command (§7.8 step 5, §7.11) — so naming it here would
   hand the operator a refusal dressed as a route (PIB-453).

   **The same limit is why abandon is not the escape from
   `archive-purge-evidence-divergent`.** That exit-6 population is divergence
   in the *archive* under a **pending** reference: a blob whose bytes do not
   hash to its name, a non-regular file where a blob belongs, or an index that
   stopped strict-decoding under an external write mid-run. (A tombstone whose
   named blob is present is **not** in that population — it holds no pending
   reference at all, and §9.3.1 classifies it by the hash's global liveness as
   either unreferenced physical residue, repaired by `--orphans --yes`, or a
   mixed tombstone/live-reference inconsistency, repaired by the confirmed
   `--blob <h> --yes`. A **retained** reference beside a present but
   non-regular or hash-wrong blob is not in it either: it holds no pending
   reference, so it is exit-3 `archive-blob-corrupt` with §9.3.1's own
   type-total removal-then-confirmed-purge route.) Every one of those lives in
   `.tpatch/features/<slug>/artifacts/intent-archive/`, which this mode
   is forbidden to touch, so directing an operator here would hand them a
   command that provably cannot consume their evidence. §9.7.2 defines that
   population's own repo-relative, executable procedure, and §10.4.1 routes it
   there and **never** to this mode (PIB-506, PIB-508).
9. **Preserves the evidence rather than deleting it.** The journal, both
   metadata preimages and every `stage-*` tree for the slug are **moved** into
   `.tpatch/local/intent-prepare/<slug>/abandoned-<12hex>/`, whose name uses the
   shipped `RandomHex12` (`internal/store/fsdurable.go:96-103`) and no clock.
   Every move is a rooted `Root.Rename` inside the held root, and the
   destination directory is created and fsynced with the rooted helpers of
   §7.7.1 — there is no path-based writer in this mode either. The report names
   that directory **repo-relative**, and states in one line that removing it is
   `rm -rf <repo-relative-dir>` and that nothing under it is tracked (PIB-271,
   PIB-272).
10. **Touches no canonical file. Ever.** No artifact, no `status.json`, no
    archive index, no blob is created, modified or removed by this mode. It is a
    `.tpatch/local/` operation only, and PIB-270 asserts the whole
    `.tpatch/features/` subtree is byte-identical afterwards.
11. **Preview by default.** Without `--yes` it prints exactly what it would move
    and exits 0 having written nothing (PIB-273). With `--yes` it performs the
    move and exits 0.
12. After it completes, the slug is ordinary: the next mutating `prepare` finds
    no journal, runs its own preflight, and refuses or proceeds on the evidence
    of the actual files. A subsequent admissible run therefore reaches exit 0
    (PIB-274, PIB-363).
13. It never removes a lock. With a process-lifetime lock there is no stale lock
    to remove (§7.4), so the one operation that could have destroyed another
    process's mutual exclusion does not exist in this design.

**The pre-abandon gate is total, and every stop in it has a route.** rev-6 said
rules 1 and 2 leave "exactly three conditions" that can stop
`--abandon-transaction` before it inspects anything. That was a count, not a
proof, and it was wrong by omission: parse, slug, workspace and read-platform
stops all precede the mutating gate, and rev-6's §10.5 additionally let the
feature-directory step refuse before the branch. rev-7 replaced the count with a
**total table**; rev-8 stated the table's domain as "the gates a *syntactically
valid* invocation can reach", which contradicted the table's own first row —
a cobra/pflag parse, arity or mutex error is precisely the stop a
**syntactically invalid** argv takes, so under that domain row 1 was
unreachable and the table failed its own reachability rule.

rev-9 fixed the domain instead of the row, and rev-10 makes it exact. The table
is total over **every argv that requests a *true* abandon, or that fails
parsing while naming the flag** — that is, every command line which either
(a) selects the mode by carrying `--abandon-transaction` or
`--abandon-transaction=true`, or (b) names `--abandon-transaction` in any
spelling and stops in the parse layer on a pflag, arity or mutex error. rev-9
said "every command line in which that flag appears", which over-reached in one
direction: `--abandon-transaction=false` is a *false* boolean value that selects
no mode at all (§5.2), so `prepare <slug> --abandon-transaction=false` is an
ordinary `generate` invocation, runs `generate`'s contract, and is **outside**
this domain — a table of abandon's stops must not claim rows over an invocation
that never enters the mode. Clause (b) keeps the domain closed under the parse
layer, so row 1 is reachable by construction
(`prepare <slug> --abandon-transaction --check`,
`--abandon-transaction --dry-run`, and a bare `--abandon-transaction` with no
slug all stop there), and it captures the mixed spellings too: cobra's mutex is
keyed on flag **presence**, not on value, so
`--check --abandon-transaction=false` also stops at row 1 and is inside the
domain by clause (b) even though clause (a) excludes it. Every other row is a
stop some parsed, mode-selecting abandon invocation reaches before rule 4
inspects a single lane entry. **Nothing else can stop it**, and PIB-511 derives
that exact row set from the implementation's control flow over that domain —
failing if a reachable stop is missing from the table, if a listed row is one no
fixture in the domain can produce, and if the domain is restated to include a
`=false` invocation that selects `generate` or to exclude the parse layer:

| # | Stop, in order | Exit / code | The route this refusal must offer |
|---:|---|---|---|
| 1 | cobra/pflag parse, arity, or a mode mutex — the stop every argv that names this flag but does not parse, or combines it with `--check` or `--dry-run`, takes, whatever boolean value it spells | `1`, pflag's own text | fix the command line; nothing was read, opened or locked |
| 2 | canonical slug grammar (§10.5 step 3) | `3`, `slug-unsafe` | supply a slug that satisfies the accepted grammar. **No lane path is named**, because the accepted no-echo rule forbids composing or echoing a path from an unsafe slug, and there is no evidence to point at until one exists |
| 3 | workspace discovery (§10.5 step 4) | `3`, `workspace-not-initialized` | **truly unavoidable**: with no workspace root there is no repo-relative lane to name and no `.tpatch/` to inspect. The remediation is to run from inside a tpatch workspace, or pass `--path`; it offers no manual removal because it cannot honestly name a target |
| 4 | read-boundary platform allowlist (§10.5 step 5) | `3`, `workspace-unsupported-platform` | **truly unavoidable**: this is the boundary that resolves the lane at all, so no procedure this document could print would be executable on that host. It states that and stops |
| 5 | mutating platform gate (§7.4.2, §10.5 step 8) | `3`, `prepare-unsupported-platform` | with evidence present: the repo-relative lane and the last-resort `rm -rf`, with its cost, exactly as the block below spells out. With an empty lane: nothing to remove, so nothing is offered |
| 6 | root-filesystem classification (§7.4.2, §10.5 step 8) | `3`, `lock-filesystem-unsupported` | identical to row 5 |
| 7 | **lock contention** (§10.5 step 9) | `3`, `transaction-in-progress` | **wait and retry — and nothing else.** No manual removal is named here even when evidence exists, because the evidence may be the *live* undo journal of a publication a sibling process is executing right now, and deleting it under that holder would destroy the undo evidence of a running transaction. The holder's identity is unknowable (§12.5), so the only safe observation is to retry (PIB-512) |
| 8 | **the authority could not be established at all** (§7.4.1, §10.5 step 9) — the workspace `*os.Root` failed to open, `root.Open(".")` failed, or `flock` failed for any non-contention reason | `3`, `directory-flock-unavailable` | identical to row 5: no lock was *taken at all*, so no holder is implied and the manual procedure is safe to name |
| — | nothing else | — | the branch runs: rules 4–13 above |

Two absences in that table are themselves normative. The **feature directory
and `status.json` are never read** in this mode, so `feature-not-found`,
`status-malformed` and `status-unreadable` cannot stop it (rule 3, §10.5 step
6). The **local-lane Git gate never runs**, so `local-lane-not-ignored` and
`local-lane-unverifiable` cannot stop it either (rule 2). A slug whose feature
directory was deleted, whose `status.json` is truncated JSON, or whose
`status.json` is unreadable is therefore still abandonable, with a corrupt
journal in the lane and no `git` on `PATH`, and PIB-509, PIB-510 and PIB-513
assert exactly that composition.

Three further §10.5 branches are absent as **post-parse** rows because this
mode's own grammar keeps them from ever being entered, not because they were
forgotten: step 1a's `--yes` validation (`--yes` is legal here, so it never
fires), step 2's `--check` handoff and step 7's `--dry-run` branch (both flags
are mutually exclusive with `--abandon-transaction`, §5.2). The last two are not
lost from the domain — an argv that pairs them with this flag is inside it and
stops at **row 1**, where a mutex error is exactly what it gets. Listing them as
separate post-parse rows would put cells in the table that no fixture can reach,
which is what PIB-511's reachability arm forbids. A fourth branch is absent for
the opposite reason: `prepare <slug> --abandon-transaction=false` is
**outside** the domain entirely, because it selects `generate` (§5.2). It is not
a stop of this mode, it takes `generate`'s whole precedence ladder, and a row
for it would claim abandon coverage over an invocation that never enters the
mode (PIB-511, PIB-537).

**Where rows 5, 6 and 8 fire and evidence exists, the refusal is itself a route
out.** All three are exit 3 and write nothing. When journal, preimage or staging
evidence is present for the slug, each of them **additionally** names the lane
and the last-resort manual procedure, so the operator is never left with a
refusal and no next action:

```text
prepare <slug>: abandon refused lock-filesystem-unsupported
  This workspace's root filesystem (<class>) is not supported for mutation,
  so tpatch cannot move the pending transaction evidence for you.
  The evidence is here, and nothing under it is tracked:
    .tpatch/local/intent-prepare/<slug>/
  Last resort, run from the workspace root:
    rm -rf .tpatch/local/intent-prepare/<slug>/
  That unblocks the slug without touching any file under .tpatch/features/.
  It also discards the undo evidence: the canonical artifacts stay exactly as
  the interrupted run left them, and `tpatch prepare <slug> --check` will
  report them truthfully. Archived bytes under .tpatch/features/ are unaffected.
```

Three properties of that text are load-bearing and each is asserted: the path
is **repo-relative** and no absolute path appears anywhere in the message
(PIB-495, PIB-497); the procedure removes only gitignored local control state
and the message says what it costs (the undo evidence) rather than presenting it
as a repair; and it is offered **only** when evidence actually exists, so an
ordinary unsupported-platform refusal with an empty lane does not suggest
deleting a directory that is not there. Row 7 of the gate table deliberately
does **not** get this text, for the reason stated there.

This is also the qualification every "exit 6 is never terminal" statement in
this document carries **for the journal and publication populations**: the
escape is reachable through the abandon mode where the environment supports
mutation, and through this named manual procedure where it does not. The one
exit-6 code abandon does not serve — `archive-purge-evidence-divergent` — has
its own named, executable procedure in §9.7.2, and §10.4.1 routes it there
instead. So the claim this document is permitted to make is that **every exit-6
population has exactly one applicable, named route out**, not that one mode
serves them all. No sentence in this PRD, ADR-035 or a shipped string claims an
unqualified "always recoverable" (§10.4, PIB-496, PIB-508).

**What it deliberately does not do.** It does not restore preimages, does not
re-run recovery, and does not decide anything about the artifacts. Restoration
from a divergent state is a human judgement — the operator has the archive
blobs (for `--regenerate`) and the moved preimage files (for the two metadata
files), both named in the exit-6 report and again in this mode's report.


## 7. The publication transaction

### 7.1 What is guaranteed, and what a filesystem cannot give

Three distinct properties are routinely conflated under the word "atomic".
This PRD applies **ADR-035 D1** directly: it separates them, claims exactly
two, and scopes the one that rev-0 overstated.

| | Property | Claimed? | Mechanism / why not |
|---|---|---|---|
| **T0** | **Instantaneous multi-file visibility.** A concurrent reader observes either the complete old set or the complete new set, at every instant. | **NO** | POSIX offers no multi-file atomic rename. Publishing six files is six independent `rename(2)` calls; between call *k* and *k+1* a concurrent reader observes a mixed set. No journal, lock or fsync changes this. Any claim otherwise would be false, so this PRD makes none. |
| **T1** | **Command-owned final verification.** At a verification point under the transaction lock, after the last rename and before the command returns, **every publication entry's current identity equals its intended new image** (exit 0) — or the command reports divergence and does not claim success. On any refusal or in-command failure, every entry equals its preimage at that same verification point, or the command exits 6 naming what it could not restore. | **YES, scoped** | Semantic CAS before every publish and every undo (§7.6), plus a final set-level verification pass under the lock (§7.8 step 13). |
| **T2** | **Crash recoverability.** After a kill, panic or power loss anywhere in the window, the *next* mutating `prepare` restores the complete old set, or refuses and preserves everything. | **YES, bounded** | The durable undo journal plus evidence-based recovery (§7.5, §7.10, §7.11), bounded by §7.11.1's explicit journal-loss boundary. |

**What T1 does not say.** It does **not** say the world is all-new after the
command returns. It says that at a point this command owns — under its lock,
after its last write — the set matched the intended images. A process that
holds no lock (an editor, `tpatch define`, `git checkout`) may write any of
those paths one microsecond later, and no postcondition of this command can
survive that. rev-0's unconditional "at the command boundary the tree is
entirely the new bytes" was therefore withdrawn: it claimed a property about
the *world*, which this command cannot own, rather than about its own final
verification, which it can.

**External writes inside the window are detected where they are observable, and
they produce divergence, not destruction.** Every entry is CAS-checked
immediately before it is published and immediately before it is undone
(§7.6). A third-party write that lands before an entry's CAS is caught and the
transaction refuses or rolls back; a write that lands after an entry's CAS but
before its rename is overwritten (the disclosed content race, §7.7.2, ADR-035
D6); a write that lands after the final verification point is simply outside
the postcondition and is stated as such. In no case does the command *undo*
third-party bytes: an undo whose CAS fails refuses that entry and exits 6 with
everything preserved (PIB-112, PIB-275, PIB-276).

**The T0 exposure window, stated exactly.** It opens at the first canonical
rename and closes at the last. Inside it a concurrent reader — `tpatch status`,
`tpatch next`, `prepare --check` in another process, an editor, `git status` —
may observe any prefix of the publication order (§7.8) applied and the rest not.
The window contains no provider call, no network I/O, no subprocess and no
user-visible prompt: it is *N* renames, *N* CAS reads and *M* directory fsyncs,
where *N ≤ 6*. That is the honest bound — a small number of syscalls, not zero
time.

**What the accepted read-only check does inside the window.** It reports what is
actually on disk, which is the truthful answer. Its own instability probes
(`docs/prds/PRD-artifact-validation-and-provenance.md:1827-1917`) may
legitimately classify a file being renamed over as `unstable`. That is correct
behavior and is not a defect of either contract (PIB-206).

### 7.2 The publication set

Computed at preflight, frozen before the journal is written, bound into the
journal by `plan_digest` (§7.5), and never recomputed. Each entry is
`(artifact_id, canonical root-relative path, action, preimage, new_image)` with
`action ∈ {create, replace}`.

| Order | Entry | Present when | Possible actions |
|---|---|---|---|
| 1 | `analysis.md` | it is generated this run | `create` (generate mode), `create`/`replace` (regenerate) |
| 2 | `spec.md` | it is generated this run | same |
| 3 | `exploration.md` | it is generated this run | same |
| 4 | `artifacts/analysis.json` | `analysis.md` is generated this run (§6.1.3) | `create` in generate mode; `create`/`replace` in regenerate |
| 5 | `artifacts/intent-archive/index.json` | at least one entry has `action = replace` — i.e. **only in `regenerate` mode** (§6.1.3) | `create`/`replace` |
| 6 | `status.json` | always, in every mutating mode that reaches publication | `replace` (or `create` for a legacy tree with no status file, which refuses earlier as `feature-not-found`) |

Every entry — including entries 5 and 6, the two *metadata* paths — carries a
full preimage **and** a full intended new image, and every entry is CAS-checked
on publish and on undo. rev-0 modelled the two metadata files as raw
side-preimages outside the entry model, which is precisely why its rollback
could not state a CAS rule for them (PIB-277, PIB-278).

Archive **blob** files are not publication-set entries: they are additive,
immutable, content-addressed and written before the window opens (§9.2). A blob
left by a crashed or refused attempt is an **orphan** — normal, harmless, and
now explicitly **reported and removable** (§9.7.3), unlike rev-0, where it was
permanent by design.

`status.json` is **last**, deliberately (ADR-035 D7). If a crash lands between entry 5 and
entry 6, the tree holds new artifacts and the old state — a strictly recoverable
combination, because recovery's undo has every preimage it needs and the state
has not yet claimed the bundle is complete. The reverse order would publish a
`defined` claim over artifacts that might never land.

### 7.3 Staging

- Root: `.tpatch/local/intent-prepare/<slug>/stage-<12hex>/`, mode `0700`,
  suffix from `RandomHex12` (`internal/store/fsdurable.go:96-103`). The G1–G3
  local-lane gate in §7.13 completes before the first write there.
- Each intended canonical output is written, synced and closed **through the
  §7.7.1 rooted control writer** — the staging lane is inside the held root and
  gets no path-based carve-out; the stage
  directory is then synced. These are **publication temporaries**, not a
  provider-transcript or attempt-history lane. On success they are removed; on
  failure only the intended canonical staged outputs and non-sensitive control
  state may remain, named by repo-relative path.
- `RetryOptions.Store` is nil. A pure generator may retain an attempt response
  **only in memory** while it validates and renders that attempt, then discards
  it. It may persist at most redacted/hashed attempt metadata (ordinal,
  validation-reason class, SHA-256 and size where needed for integrity); it
  never persists a raw attempt/transcript, prompt, corrective prompt, excerpt,
  source-body sink or `raw-*-response-*.txt` in failure residue, reports,
  archive or journal. The intended canonical staged output is the sole
  publication temporary and may equal the provider result. ADR-027 D2/D3 is
  the hard boundary.
- Cleanup retains no raw response. `--abandon-transaction` moves only allowed
  control/staged-output evidence. A future raw capture needs a superseding ADR;
  this PRD authorizes none. This is a structural sink rule: a final intended
  canonical staged output may legitimately equal a provider response byte for
  byte, so no guard may assert that provider-byte substrings occur nowhere.
  Guards instead prove `RetryOptions.Store == nil`, no retry/raw-attempt or
  transcript sink/path/type exists, and no report/history/journal/archive
  receives an attempt payload outside the allowed canonical publication
  temporary (PIB-188…PIB-190, PIB-376, PIB-419, PIB-420, PIB-446).

### 7.4 Lock: one held workspace-root directory authority

#### 7.4.1 Authority, scope and acquisition

A mutating invocation constructs one explicit `WorkspaceAuthority` containing
**both** the workspace `*os.Root` and the directory `*os.File` returned by
`root.Open(".")`. Both pointers remain strongly reachable in that authority for
the complete invocation; an integer fd is never retained as the authority.
On Linux and Darwin acquisition calls `lockFile.SyscallConn().Control`, performs
the one `syscall.Flock(LOCK_EX|LOCK_NB)` while that control callback owns the
descriptor, and returns the still-held `*os.File`. The locked inode is therefore
the same workspace-root object used for rooted I/O; there is no second locator
to bind.

There is no lock file, cache file, cache key, cache directory, `HOME`, XDG,
`LocalAppData`, `os.UserCacheDir`, pid, slug-derived authority, cleanup policy
or durable lock residue. `flock` success is authority. Only `EWOULDBLOCK` or
`EAGAIN` is contention (`transaction-in-progress`, exit 3); every other result
is fail-closed `directory-flock-unavailable` (exit 3) before mutation. The
human and JSON remediation is: **"Move the workspace to a supported local
filesystem, then retry."** It names the detected filesystem class but never an
absolute pathname.

**Failing to obtain the handles is the same refusal, and it is classified
rather than left to a generic error.** Two steps precede the classification and
the lock: `os.OpenRoot` on the workspace root, and `root.Open(".")` for the
directory `*os.File`. Either can fail — the root was renamed or removed between
discovery and open, a permission or `EMFILE`/`ENFILE` limit denies it, the path
resolved to a non-directory. rev-8 left those failures unclassified, which is
the one way this fail-closed ladder could still surface an unnamed error to an
operator. They are **`directory-flock-unavailable`, exit 3**, before any
mutation and before any `fstatfs` or `flock` is attempted: the authority could
not be established, no lock was taken, and no holder is implied — which is
exactly the meaning that code already carries for a non-contention `flock`
failure. It is deliberately **not** `transaction-in-progress` (nothing was
observed to be held), not `lock-filesystem-unsupported` (no class was
classified) and not `workspace-not-initialized` (discovery already succeeded).
Because no holder is implied, this refusal is safe to accompany with §6.6's
last-resort route: when journal, preimage or staging evidence exists for the
slug, it additionally names the repo-relative lane and the manual removal, and
§6.6's pre-abandon gate table carries it as **row 8** together with the
non-contention `flock` failures (§6.6, §10.4.1, PIB-536).

The scope is the whole workspace, not a slug. Every mutating `prepare` mode
(`generate`, `manual`, `regenerate`, `abandon`) and every archive
`purge --yes` serializes across all slugs in that workspace. The invocation
acquires once, threads this one authority through recovery, publication, nested
archive helpers and purge, and never recursively re-acquires. Nested helpers
receive the authority, never an fd and never reacquire. The **never-acquire**
population is closed and stated positively: `prepare --check`, every
`--dry-run`, archive `list`, **every archive `purge` preview — that is, `purge`
on any selector without `--yes`, including the pending-hash and pending-journal
cases of §9.7** — and `doctor` never acquire it; no diagnostic command opens or
locks the workspace root at all (§12.5). Only the `--yes` form of `purge` takes
the authority, which is what makes the confirmation flag, and not the verb, the
thing that serializes (§7.12, PIB-490, PIB-515, PIB-530).

Release is explicit and exactly once at invocation end: issue `LOCK_UN` through
the retained file's `SyscallConn().Control`, close that `*os.File`, then close
the retained `*os.Root`; `runtime.KeepAlive(authority)` occurs at that release
boundary. A finalizer is forbidden from unlocking or closing either member of a
live authority. Closing remains a last-resort kernel release on process death,
not normal ownership transfer. Forced-GC real-process contention and a fixture
which drops the strong `*os.File` reference are mandatory (PIB-433…PIB-435).

**`SyscallConn.Control` versus release/`Close` — the accepted concurrency
rule.** `(*os.File).SyscallConn()`'s `Control` holds a reference on the file's
descriptor for the duration of its callback, so a `Close` cannot pull the
descriptor out from under an in-flight `flock`. It does **not** make `Control`
and `Close` orderable from different goroutines: a `Control` that begins after
a `Close` has started returns an error (`ErrClosed`/`ErrFileClosing`) instead of
running its callback. This design accepts that rather than adding a mutex, and
bounds it with one ownership rule: **the authority is owned by the single
invocation goroutine; acquisition, every threaded use, and the one release all
happen on it.** Two consequences are normative and asserted rather than
assumed:

- No other goroutine may close, unlock or re-`Control` a live authority. There
  is no concurrent-release path in the design, so the only way to reach the
  racing case is a bug.
- A `Control` error after release is a **programming error**, never evidence
  about the lock. It may not be reported as `transaction-in-progress`, may not
  be read as "the lock was lost", and may not be read as an acquisition. It
  fails closed as `directory-flock-unavailable` if it can be reached at all,
  and PIB-481 is the deterministic fixture that pins this (release-then-Control
  and a concurrent-close attempt both produce the stated outcome).

Only provider generation obeys `--timeout` / `--timeout-phase`. Filesystem
inspection, Git, recovery, publication, fsync and release have no hard
wall-clock bound, and this PRD makes no total-command or total-lock-hold
promise. The publication window contains no provider, Git subprocess or prompt,
but that fact is not a termination bound.

#### 7.4.2 Platform and filesystem gate

Only Linux and Darwin mutating builds provide this authority.
**Classification happens on the descriptor the command already holds, not on a
pathname before opening, and it goes through the same `SyscallConn` discipline
as the lock.** The order is: open the workspace `*os.Root`; open `"."` through
it to obtain the directory `*os.File`; call
`dirFile.SyscallConn()` and, **inside its `Control` callback**, run
`syscall.Fstatfs(int(fd), &buf)` on the descriptor that callback owns;
classify from the returned buffer after the callback returns; then, in a second
`Control` callback on the same retained file, attempt the single nonblocking
`flock`. rev-4 said the classification happened "before opening a workspace
root for mutation", which describes a path-based `statfs(2)` on a name that a
concurrent rename could have redirected between the check and the open. Using
`fstatfs` on the held descriptor makes the classified object and the locked
object **the same inode by construction**, with no TOCTOU window between them
(PIB-478).

**`(*os.File).Fd()` is forbidden in the authority path.** rev-5 wrote the
classification as `syscall.Fstatfs(int(dirFile.Fd()), &buf)`, which contradicts
the rule §7.4.1 states for the flock on the same file. `Fd()` yields a bare
integer that carries no reference to the `*os.File`: nothing about it prevents
the file being closed and the descriptor number reused between the call and the
syscall, which is precisely the naked-fd hazard the authority design rejects.
`SyscallConn().Control` holds a reference for the duration of its callback, so
the descriptor cannot be pulled out from under either syscall. The rule is
therefore uniform and mechanically checkable: **every syscall in the authority
path — `fstatfs`, `flock` and the unlock — runs inside a `Control` callback on
the retained `*os.File`, and no `(*os.File).Fd()` call appears anywhere in
`internal/intentlock`.** The shipped `rescap` lock does use `int(f.Fd())`
(`internal/rescap/lock_unix.go:80`, C176); it is a precedent this PRD
deliberately does not copy, and §7.4.4's non-extraction rule is what makes that
divergence legal without changing `rescap` (PIB-504).

The denied sets are exact and are matched by exact value or exact string, never
by prefix, suffix or substring:

| Platform | Denied, exactly | Match rule |
|---|---|---|
| Linux (`Statfs_t.Type`, normalized to `uint32` as the shipped helper does, `internal/rescap/statfs_linux.go:57-61`) | `0x6969` `NFS_SUPER_MAGIC` (NFS and NFSv4 share it), `0x517B` `SMB_SUPER_MAGIC` (legacy smbfs), `0xFF534D42` `CIFS_MAGIC_NUMBER`, `0xFE534D42` `SMB2_MAGIC_NUMBER`, `0x65735546` `FUSE_SUPER_MAGIC` (`fuse` and `fuseblk` share it) | exact numeric equality |
| Darwin (`Statfs_t.Fstypename`, the `[16]int8` array converted and trimmed at the first NUL exactly as `internal/rescap/statfs_darwin.go:39-49` does) | `nfs`, `smbfs`, `webdav`, `macfuse`, `osxfuse` | exact string equality |

Two Darwin facts are pinned rather than assumed. First, `f_fstypename` is at
most 15 bytes plus a NUL, so it is a **kernel-assigned short name**, not a mount
description; the subtype an operator sees in `mount` output lives in
`f_mntfromname` and is not consulted. Second, macFUSE reports `macfuse` on 4.x
and `osxfuse` on 3.x, but a third-party filesystem built on it may report its
own name (`sshfs`, `gocryptfs`, …). Those names are **not** denied — no
`fuse`-prefix or `fs`-suffix pattern is applied, because a pattern over a
15-byte namespace would deny unrelated local filesystems and would still miss
arbitrary names. Unrecognized names take the unknown-local route below
(PIB-479).

A denied class refuses `lock-filesystem-unsupported` (exit 3), naming the
detected class and saying to move the workspace only for that explicit denial
(or a failed classification/flock).

This is a denylist, not rev-3's local allowlist: overlayfs remains admissible,
as do exFAT, ZFS, f2fs and otherwise unknown local classes. **A non-denied
class is not an approval — it is a deferral to the real lock.** After a class is
not denied, successful real nonblocking flock is still required; every
non-contention flock error fails closed as `directory-flock-unavailable`.
Statfs/classification failure also fails closed.

**Disclosed limits of the non-denied route.** An unrecognized local filesystem
can *lie about `flock`*: it can accept `LOCK_EX|LOCK_NB` and return success
while providing no real mutual exclusion (a network or user-space filesystem
that reports a local-looking name, a FUSE filesystem whose kernel name is its
own, some FAT-family and container-layer drivers). A successful flock is
therefore evidence that the kernel accepted the operation, not proof that a
second process would be excluded; where the filesystem is honest — every denied
class is excluded and the shipped Linux/macOS CI classes are honest — exclusion
holds. The check deliberately does not classify nested mounts below the root and
does not claim that every write target's filesystem was classified; all tpatch
mutators nonetheless contend on the one root-directory authority. **No
cross-machine guarantee follows from any of this**: `flock` is per-host, two
machines sharing a filesystem are not serialized by it, and nothing in this
design detects that case. This limitation is disclosed in docs and help text,
never softened (PIB-480).

Windows, BSD, Solaris, AIX, Plan 9, WASI, JS and future unmatched targets
refuse `prepare-unsupported-platform` (exit 3). The accepted read-only
`prepare --check` envelope remains `unix || windows` and is untouched. Native
two-process contention-and-release CI is required on typical Linux and macOS
runners, alongside classification fixtures for denied and non-denied classes
(PIB-416, PIB-442, PIB-478…PIB-480).

#### 7.4.3 Identity, aliases, rename and replacement boundary

Immediately after `root.Open(".")`, the command captures native identity from
the retained directory `*os.File` (device+inode on Linux/Darwin). This follows
the Go 1.26 `os.Root` documentation, by symbol (`$GOROOT/src/os/root.go`): on
these platforms a Root holds a directory descriptor and continues to reference
that directory when it is moved. Alias paths known at acquisition can therefore
contend on the same inode.

The original discovered pathname cannot, however, be portably rediscovered
after a root rename. Holding the old root protects the original inode; it does
**not** reveal its new pathname. Before the publication window and again at
final verification, the command resolves only the original discovered live
pathname and compares it to the held identity. If that pathname is missing or
names another inode, it refuses `workspace-root-changed` (exit 5) before the
window, or preserves evidence and exits 6
`workspace-root-replaced-after-publication` after publication began. It never
returns 0 for an unlinked/moved old root and never claims rename success.

Deletion/recreation and rename both make the original pathname unavailable or
different; a new directory can host a separate lock. The checks detect this
boundary at their stated points but cannot discover the moved root. Rooted
`os.Root` operations may continue against the original directory across rename;
that fact is intentionally separate from live-path discoverability and rooted
confinement (PIB-413…PIB-415, PIB-436).

#### 7.4.4 Rejected alternatives and executable cases

A lock/cache file, user-cache authority, `.git` authority and named semaphore
are rejected because each adds a pathname or external namespace that can split,
be cleaned, be unavailable, or need lifecycle policy. POSIX record locks
(`fcntl`) are rejected for close-any-descriptor semantics in a multithreaded
process. `rescap`'s file lock is a precedent, not an extraction or reuse: a
file-lock helper does not implement this directory-descriptor authority. No
sentence in this PRD or ADR-035 claims the prepare authority is extracted from,
moved out of, or shared with `rescap`; S1b instead owns a prepare-specific
helper that opens the root, opens `"."` through it, `fstatfs`-classifies that
held descriptor, captures identity and flocks it. `rescap` behavior and bytes
remain unchanged (PIB-482).

Real-process coverage is required: same-workspace different-slug contention;
process death; forced GC while held; self-reentry rejection without a second
flock; alias-at-acquisition contention; renamed/missing original path refusal;
deletion/recreation before the window; deletion or recreation after a rename
with the exit-6 evidence path; denied/non-denied filesystem classes; the
`fstatfs`-on-held-descriptor ordering; the release-then-`Control` rule; and
unavailable flock results. These are PIB-395…PIB-398, PIB-410…PIB-418,
PIB-433…PIB-442, PIB-478…PIB-481 and PIB-504.

### 7.5 Journal: location, schema, binding, and why it is undo-only

**Location**: `.tpatch/local/intent-prepare/<slug>/journal.json`, mode `0600`,
written with the **prepare-owned rooted durable single-file helper** of §7.7.1
and read with a **strict** decoder — the discipline at
`internal/cli/land_journal.go:348-380` and
`internal/store/resource_publish.go:305-320`.

**The journal is not written with `gitutil.DurableWriteFile`.** rev-4 named that
shipped helper as the journal writer. It is the right *durability shape* — temp,
write, fsync, close, rename, directory fsync — and it is why `land`'s journal
survives a crash (`internal/gitutil/index_snapshot.go:455-500`). But it takes a
directory **pathname** and creates its temp with `os.CreateTemp`, so calling it
would put a path-based writer inside a command whose D2 rule is that every
mutating write is handle-relative. A local-lane carve-out from the rooted
guarantee is not available: `.tpatch/local/` is inside the same held root, is
reached through the same ancestor chain, and an ancestor redirect there
redirects the undo evidence for a canonical publication. The journal, both raw
metadata preimages (§7.6.2), every staged file (§7.3), the `abandoned-*`
evidence moves (§6.6) and every other `.tpatch/local/` write therefore use the
§7.7.1 rooted helper. `DurableWriteFile` and `writeFileAtomic*` remain
unchanged for their shipped callers and are never called from a mutating
`prepare` path (§7.7.3, PIB-454…PIB-456).

**Location rationale**: gitignored `.tpatch/local/`, because the journal is
transient control state, is machine-local, and must never enter a commit. It
carries **no artifact content** except two small machine-written preimages
(§7.6.2). §7.11.1 states the boundary this location creates.

```json
{
  "version": 1,
  "slug": "fix-model-id-translation",
  "mode": "regenerate",
  "run_nonce": "9f13a0c4b7e25d81",
  "plan_digest": "6b1f…",
  "stage_rel": ".tpatch/local/intent-prepare/fix-model-id-translation/stage-a1b2c3d4e5f6",
  "entries": [
    {
      "artifact_id": "analysis",
      "rel": ".tpatch/features/fix-model-id-translation/analysis.md",
      "action": "replace",
      "preimage": { "exists": true, "sha256": "…", "size": 4211, "mode": 420 },
      "preimage_blob": "a91e…",
      "new_image": { "exists": true, "sha256": "…", "size": 5017, "mode": 420 },
      "staged_rel": ".tpatch/local/intent-prepare/…/stage-…/analysis.md"
    },
    {
      "artifact_id": "archive_index",
      "rel": ".tpatch/features/fix-model-id-translation/artifacts/intent-archive/index.json",
      "action": "replace",
      "preimage": { "exists": true, "sha256": "…", "size": 512, "mode": 420 },
      "preimage_raw_rel": ".tpatch/local/intent-prepare/…/index.preimage.json",
      "new_image": { "exists": true, "sha256": "…", "size": 883, "mode": 420 },
      "staged_rel": ".tpatch/local/intent-prepare/…/stage-…/index.json"
    },
    {
      "artifact_id": "status",
      "rel": ".tpatch/features/fix-model-id-translation/status.json",
      "action": "replace",
      "preimage": { "exists": true, "sha256": "…", "size": 604, "mode": 420 },
      "preimage_raw_rel": ".tpatch/local/intent-prepare/…/status.preimage.json",
      "new_image": { "exists": true, "sha256": "…", "size": 641, "mode": 420 },
      "staged_rel": ".tpatch/local/intent-prepare/…/stage-…/status.json"
    }
  ]
}
```

**Strict decode and binding — a journal is used only when it proves it is
ours.** Every one of these is checked before a single byte is restored, and any
failure refuses with exit 6, preserves everything, names the journal path and
points at §6.6's abandon route (PIB-298 … PIB-305):

| # | Bind | Failure code |
|---|---|---|
| J1 | exactly one JSON value, then EOF; no trailing content | `journal-corrupt` |
| J2 | no unknown field at any nesting level | `journal-corrupt` |
| J3 | `version` equals this build's constant — refused, never guessed (`internal/cli/land_journal.go:56-58` is the precedent) | `journal-version-mismatch` |
| J4 | `slug` equals the invocation's validated slug | `journal-foreign` |
| J5 | `mode` is in the closed set `{generate, manual, regenerate}`; `manual` is impossible and refuses | `journal-corrupt` |
| J6 | `run_nonce` is exactly 16 lowercase hex bytes | `journal-corrupt` |
| J7 | every `rel` is a `fs.ValidPath` root-relative name inside `.tpatch/features/<slug>/`, and every `preimage_raw_rel` / `staged_rel` is inside that slug's `.tpatch/local/` lane — the shipped containment check (`internal/cli/land_journal.go:383-415`) | `journal-path-escape` |
| J8 | `plan_digest` equals the SHA-256 recomputed over the canonical encoding of `entries` (ids, paths, actions, preimages, new-images) | `journal-forged` |
| J9 | every entry's `artifact_id` is in the closed six-value set and appears at most once | `journal-corrupt` |
| J10 | `entries` is non-empty and is not `null` — the shipped no-null-arrays rule (`internal/store/resource_publish.go:305-320`) | `journal-corrupt` |

J8 is what makes forgery pointless: an attacker who edits an entry's preimage
to point at content they control invalidates the digest, and recovery refuses
rather than restoring attacker-chosen bytes over a canonical file. A journal
that survives J1–J10 still cannot cause a write, because every restore is
additionally CAS-gated on the file's *current* identity (§7.6).

- **No wall-clock field.** ADR-027 D6 and ADR-033 D10 forbid it in tracked
  artifacts; this PRD applies the same rule to the journal so that two
  identical operations produce identical journals, which is what makes
  PIB-160's determinism test possible. `land`'s journal does carry
  `created_at` (`internal/cli/land_journal.go:109`); this design deliberately
  does not, and ADR-035 D5 records the divergence and its reason (recovery
  decides from evidence, never from time).
- **No content**, with two enumerated exceptions (§7.6.2).
- **`Phase` is deliberately absent.** `land` keeps one and marks it advisory
  (`internal/cli/land_journal.go:110-111`); this design omits it entirely so
  no reader can be tempted to decide from it.
- **`run_nonce` is not lock ownership.** It exists only so a report can name
  which interrupted run produced the state an operator is looking at. Lock
  ownership is the kernel's, not a field's (§7.4).

**Undo-only, and why.** `land` must roll *forward* because `git commit` already
advanced HEAD, an irreversible act (`internal/cli/land_journal.go:11-23`).
Nothing in a prepare publication is irreversible: the new artifacts did not
exist before, and regenerating them is a re-runnable operation. So recovery
never completes a partial publication. It restores the old set and lets the
operator re-run. This is simpler, has strictly fewer failure modes, and cannot
publish a bundle whose staged half was pruned (ADR-035 D5).

### 7.6 Identity, and the semantic compare-and-swap

#### 7.6.1 Identity

Identity is `(exists, sha256, size, mode)`; equality requires all four (or both
non-existent).

**Citation correction.** The shipped `landJournalFileState` is
`(exists, sha256, mode)` — it carries **no size**
(`internal/cli/land_journal.go:65-79`). rev-0 claimed size was part of the
shipped shape; it is not. This design **extends** the shipped shape with
`size`, deliberately: size is a cheap mismatch signal that catches truncation
and growth before a hash is computed, and it is *compared*, never *trusted* —
equality still requires the SHA-256 (PIB-306).

- **Canonical artifact preimages** are recovered from the intent archive
  (§9.2): the blob is written and fsynced *before* the journal is finalized, so
  by the time the window can open, every replaceable byte already has a durable
  copy at a content-addressed path. The journal stores the blob's hash, not its
  bytes.
- New-image identities are computed from the staged files after they are
  fsynced, so the journal cannot describe bytes that are not durable.

#### 7.6.2 The two raw metadata preimages

`index.json` and `status.json` prior bytes are stored raw under the journal
directory (`index.preimage.json`, `status.preimage.json`, `0600`, gitignored,
removed with the journal), because neither belongs in a tracked,
content-addressed *intent* archive: archiving the archive index in the archive
is circular, and `status.json` is lifecycle metadata, not intent content
(ADR-035 D11).

**They are still full publication-set entries** with `preimage`, `new_image`
and CAS treatment identical to every artifact entry (§7.2). The raw file is
*where the bytes live*, not *whether the entry is modelled*. This is the rev-0
gap that made metadata rollback unspecifiable.

#### 7.6.3 The CAS rule — total over every mutating step

Before **every** step that changes a byte, the target's current identity is
recomputed through the held root and compared:

| Step | Expected current identity | On mismatch |
|---|---|---|
| publish entry *i* (`create`) | **absent** | abort before the rename; exit 5, `entry-appeared`; roll back what is published |
| publish entry *i* (`replace`) | equals `preimage` | abort before the rename; exit 5, `entry-changed`; roll back what is published |
| undo entry *i* (`create`) → remove | equals `new_image` | **refuse that entry**; exit 6, `undo-cas-mismatch`; the file is left exactly as found |
| undo entry *i* (`replace`) → restore | equals `new_image` | **refuse that entry**; exit 6, `undo-cas-mismatch`; the file is left exactly as found |
| restore `index.json` / `status.json` | equals that entry's `new_image` | same as above |
| `--manual` `status.json` rename | equals the rooted status preimage immediately before rename | exit 5, `status-changed`; no rename or derived-index refresh |
| write archive blob `<h>.blob` | absent, **or** present with bytes hashing to `<h>` | exit 3, `archive-blob-corrupt`; never overwritten |
| publish/rehydrate `index.json` | equals the index entry's `preimage` | exit 5, `archive-index-changed`; nothing written |
| purge `index.json` | strict-decoded raw identity equals the captured purge index preimage immediately before rooted rename | exit 3, `archive-purge-index-changed`; remove no blob |
| purge a blob (§9.7) | present with bytes hashing to its name | exit 3, `archive-blob-corrupt`; nothing removed |

Two invariants follow, and both are asserted rather than asserted-about:

1. **A concurrent edit is never clobbered by an undo.** If a third party
   rewrote a published file, its identity no longer equals the new image, so
   the undo refuses and the bytes survive (PIB-112, PIB-275).
2. **A forged or corrupt journal cannot cause a write.** J1–J10 refuse it
   before recovery starts, and even a journal that passes them cannot restore
   over a file whose current identity does not match the exact image that step
   expects (PIB-303, PIB-307).

The precedent is shipped: `PublishBatch` compares the *semantic body* of an
existing immutable file before deciding to reuse or refuse it, and never
rewrites it in place (`internal/store/resource_publish.go:230-285`,
`internal/store/resource_publish.go:358-399`).

### 7.7 Rooted publication writes

#### 7.7.1 Every write is handle-relative

rev-0 performed rooted *reads* and then a path-based `os.Rename`, leaving an
ancestor-swap race: a directory component could be replaced between the check
and the rename, and the rename would then land outside the intended tree. The
pinned toolchain (`go 1.26.1`, `go.mod:3`) provides `(*os.Root).Rename`, so
that gap has no reason to exist.

| Operation | Primitive | Note |
|---|---|---|
| create a temp file in the destination directory | `(*os.Root).OpenFile(tmpRel, O_CREATE\|O_EXCL\|O_WRONLY, 0600)` | `O_EXCL` so a pre-existing temp name never gets written through; name is `.<base>.tmp-<12hex>` inside the destination directory, so the rename is same-filesystem (the shipped rationale at `internal/store/store.go:884-886`) |
| publish | `(*os.Root).Rename(tmpRel, destRel)` | replaces the destination in one syscall; never follows a final-component symlink |
| pre-publish CAS read | `(*os.Root).Lstat` + the accepted bounded rooted capture | ADR-034 D1/D8/D9, unchanged |
| create a directory | rooted component-by-component `(*os.Root).Mkdir` helper | `0755` tracked, `0700` local; no path-based `MkdirAll` |
| remove (undo of a `create`, temp cleanup) | `(*os.Root).Remove` | |
| fsync a directory | `(*os.Root).Open(dirRel)` then `Sync()` | best-effort on platforms where directory fsync is not meaningful, matching the shipped writer (`internal/store/store.go:910-914`) |

**No `os.Rename`, `os.CreateTemp`, `os.WriteFile`, `os.Remove`, `os.MkdirAll` or
`filepath.Join(repoRoot, …)` appears in a mutating prepare write path.** PIB-308
source-scans for exactly that, and PIB-309 is its sensitivity fixture.

**The rooted durable single-file helper — the one control writer.** Every
single-file write this command performs *outside* the entry-by-entry
publication loop — the journal, `index.preimage.json`, `status.preimage.json`,
each staged canonical output, the `--manual` status publication, and every
other `.tpatch/local/` control or metadata file — goes through one
prepare-owned helper with a fixed, testable shape:

1. `(*os.Root).Mkdir`-chain the destination directory if it does not exist, at
   `0755` in the tracked tree and `0700` in `.tpatch/local/`, fsyncing each
   created component through the root.
2. `(*os.Root).OpenFile(tmpRel, O_CREATE|O_EXCL|O_WRONLY, perm)` where `tmpRel`
   is `.<base>.tmp-<12hex>` **in the destination directory**, so the later
   rename is same-filesystem (the shipped rationale at
   `internal/store/store.go:884-886`); `perm` is `0600` for local-lane and
   metadata files, `0644` for new tracked files, and the preserved bits for a
   replacement.
3. Write the complete bytes, `Sync()`, `Close()`.
4. **Immediate CAS where the step has an expected current identity** — the
   `--manual` status preimage (§6.2), the archive index preimage (§9.7.2), and
   every publication entry (§7.6.3). The re-capture is a rooted `Lstat` plus the
   accepted bounded rooted read through the *same* held root, taken immediately
   before step 5 and never earlier. A step with no expected identity (a fresh
   journal, a fresh staged file) skips only this sub-step, not the rest.
5. `(*os.Root).Rename(tmpRel, destRel)`.
6. `(*os.Root).Open(dirRel)` then `Sync()` on the destination's parent
   directory, best-effort exactly as the shipped writer treats it
   (`internal/store/store.go:910-914`).

On any failure the temp is removed with `(*os.Root).Remove` and the error is
returned; a partially written temp is never renamed. The helper takes the
`WorkspaceAuthority` (§7.4.1) and root-relative names only — it has no
parameter that can carry an absolute path, which is what makes "no path-based
control write exists" a type-level property rather than a call-site discipline
(PIB-454, PIB-455, PIB-456).

#### 7.7.2 What this closes, and what it does not

- **Closed: escape from the held root.** `*os.Root` resolves the supplied
  relative name beneath its held root and rejects a resolution that would leave
  it. A raced ancestor replacement cannot redirect this publication outside the
  workspace root (PIB-310).
- **Not closed: in-root pathname redirection.** `os.Root` may follow a relative
  symlink that remains inside the root. An attacker can redirect `features/` or
  `<slug>/` to a different in-root directory after preflight; a rooted rename
  can then target that in-root alias. This is not an outside-root escape and no
  sentence may call it impossible. Pre-rename identity CAS detects the ordinary
  case where the resolved target differs from the preimage; it **cannot** prove
  that the canonical pathname designated the same object throughout, and cannot
  detect an in-root alias whose resolved regular file has exactly the expected
  preimage. PIB-406 exercises the detected redirect and guards the disclosure.
- **Closed only for the final leaf: writing through a final symlink.** The
  pre-rename rooted `Lstat` gate refuses a final symlink, junction, directory,
  FIFO, socket or device. This says nothing about a redirected ancestor.
- **Not closed: content identity.** Between an entry CAS and its rename, a
  concurrent writer can replace the resolved destination bytes; the rename can
  overwrite them. That is a residual content race. The in-root redirection
  limit above is a separate name-to-object residual; neither is described as a
  "closed pathname half" (ADR-035 D6).
- **Not used: the documented `Root.Chmod`, `Root.Chown`, and `Root.Chtimes`
  caveat.** This design calls none of them (PIB-311).

#### 7.7.3 The shipped single-file writers are not reused by `prepare`

`writeFileAtomicWithRename` (`internal/store/store.go:878-917`) creates its temp
with `os.CreateTemp` and renames path-based, so it remains unchanged for shipped
callers and is never a `prepare` writer. `gitutil.DurableWriteFile`
(`internal/gitutil/index_snapshot.go:455-500`) has the same standing: it is the
durability shape the journal copies and the writer `land`'s journal keeps using,
but it takes a directory pathname and creates its temp with `os.CreateTemp`, so
it is never a `prepare` writer either — including for the gitignored local
lane, which gets no carve-out from D2 (§7.5).

Both transactional publication and the single-file `--manual` status
publication use the rooted temp/identity/CAS helper in §7.7.1 and §6.2. The
only non-transactional `FEATURES.md` refresh is also routed through a rooted
best-effort helper; its derivation/T1 carve-out, not a path-based exemption, is
§12.3.1. PIB-312 pins the shipped helpers and their old callers; PIB-313 proves
no mutating `prepare` path calls `writeFileAtomic`/`SaveFeatureStatus`, and
PIB-454 extends the same proof to `gitutil.DurableWriteFile`.

### 7.8 Publication order, CAS points, fsync and durability

Ordered algorithm. Every step is durable before the next begins.

**Mutating pre-step (no mutation):** after the separate dry-run branch has
returned (§6.4, §10.5), discover/validate the workspace and platform, open the
held workspace root and `fstatfs`-classify its root-directory descriptor
(§7.4.2). A denied or unclassifiable filesystem, or an unsupported mutating
platform, refuses here — before the lock and before any byte. A failure to open
the root or its directory handle at all refuses `directory-flock-unavailable`
here for the same reason (§7.4.1).

1. Acquire the one held-root directory lock (§7.4). Contention → exit 3, `transaction-in-progress`.
2. **`--abandon-transaction` branches here** and returns (§6.6). The branch is
   after the lock — so it never races a live sibling — and **before** the Git
   privacy gate, the journal recovery and the pending-hash gate, so the mode
   never depends on a
   working `git` to move bytes that already exist in the lane, and never runs
   the automatic recovery that would consume, undo or delete the very evidence
   it was invoked to move. It is also reached **without** the feature-directory
   and `status.json` step of §10.5 step 6, which does not run in this mode at
   all. No step below runs in `abandon` mode (PIB-452, PIB-494, PIB-510).
3. **Local-lane Git gate (every mode except `abandon`).** Run G1, then the
   applicable G2/G3 gate, and G4 for `--regenerate` (§7.13). An unverifiable
   Git/privacy state refuses here — before the first local-lane or canonical
   byte, which is the only property the gate needs. Holding the lock across
   this gate writes nothing and costs only exclusion; it is what lets step 2
   sit above it.
4. **Journal recovery (§7.11), and it is terminal.** If a journal exists for the
   slug, recover it and **return**: exit 0, `outcome: "recovered"`, naming what
   was restored plus the sanitized retry for the operation the operator
   originally asked for. Divergent evidence (`recovery-divergent`), a J1–J10
   bind failure (`journal-*`) and a refused undo CAS (`undo-cas-mismatch`) are
   each exit 6 with everything preserved; §7.9's exit-5 `rolled-back` outcome
   belongs to in-command rollback, not to recovery. No step below runs in that
   invocation.
   If no journal exists, stale `stage-*` trees for the slug are removed as
   ordinary lane hygiene — that is not a recovery and does not terminate — and
   evaluation continues.
5. **Pending archive-hash state refuses; it is not recovered here.** If the
   index holds removal-pending references for the slug, this invocation
   **refuses**: exit 3, `recovery-pending`, with the whole tree byte-identical
   — no blob removed, no tombstone published, no index rewrite. A pending
   reference makes the purge transaction the **global owner** of that content
   hash (§9.3), and ownership outranks every other classification of the same
   hash: if `h` has any pending reference, an observation that would otherwise
   be reported as dangling, residue, mixed or corrupt on `h` is reported as
   `recovery-pending` and routed to the owner instead, because the transaction
   in flight is what decides `h`'s next state. It names the
   sanitized `tpatch feature intent-archive purge <slug> --all --yes` with
   `retry_cwd: "workspace-root"` (or, when every pending reference shares one
   hash `h` and the refusal can therefore prove the narrower selector is total
   over the observed pending set, `--blob <h> --yes`). No step below runs. If
   nothing is pending, evaluation continues (§7.11, §9.7.2, PIB-525, PIB-526,
   PIB-539).
6. Inspect (accepted inspector, ADR-034 boundaries). Compute and freeze the
   publication set. Refuse here for every §6 admissibility failure — nothing
   has been written.
7. Stage and validate generated bytes (§7.3, §11.6). Any failure here aborts
   with zero canonical mutations.
8. **Set-level revalidation**: re-resolve/live-identity-check the workspace root
   against the held descriptor, then compare every entry's canonical identity vs
   preflight. Any mismatch → abort, exit 5, before anything is archived.
9. Scan every `replace` entry's prior bytes under the redaction contract
   (§9.6). A match refuses the invocation (exit 3) before any blob exists.
10. Write archive blobs for every `replace` entry; fsync each; fsync `blobs/`.
    Capture `index.preimage.json` and `status.preimage.json`; fsync them.
11. Write the journal; fsync it; fsync the journal directory. **The window is
    now armed.**
12. For each entry in the fixed order of §7.2: **CAS the entry** (§7.6.3), then
    `(*os.Root).Rename` staged → canonical, then fsync the entry's parent
    directory.
13. **Final verification** (T1's owning point): re-resolve/live-identity-check
    the workspace root against the held descriptor, then re-read every entry's
    identity and require equality with its new image. A root mismatch is
    `workspace-root-replaced-after-publication` (exit 6); another mismatch is
    `post-publication-divergence` (exit 6). Nothing is undone because that could
    destroy third-party bytes (PIB-314, PIB-315, PIB-415).
14. Clear the journal (remove `journal.json`, both preimage files and the
    staging tree; fsync the journal directory).
15. Explicitly unlock the retained directory `*os.File` through
    `SyscallConn.Control`, close it, close the held root and call
    `runtime.KeepAlive(authority)` at the release boundary. Nothing durable
    remains.

Step 14 is the point after which the transaction is invisible. Steps 11→12 are
the armed window; step 12 is the T0 exposure window (final T1 verification is step 13).

**Why step 4 returns instead of continuing.** rev-5 recovered and then
fell through into steps 6 and beyond, where a lifecycle, admissibility,
coherence, provider or archive gate can refuse with exit 2 or exit 3. That
produced an invocation which had already written — recovery restores canonical
bytes and removes the journal — and then returned an exit code whose whole
contractual meaning is "wrote nothing" (§10.4). Two repairs were available:
qualify exit 3 as "zero-write except after recovery", or make recovery
terminal. This PRD takes the second, because the first makes the one exit code a
harness can depend on conditional on state the harness cannot observe. The cost
is one extra invocation after an interrupted run, and it is paid explicitly:
the report says what was recovered and hands over the exact command to run next
(§7.11, §10.2). There is no hidden auto-continue and no flag that restores
one (PIB-483, PIB-484, PIB-487).

**Why step 5 refuses instead of recovering.** rev-6 and rev-7 had *two*
commands run the pending-hash state machine of §9.7.2 — a mutating `prepare`
and `feature intent-archive purge --yes`. Both then terminated, so exit 3's
zero-write promise was safe; but the pending-purge transaction had two owners,
and every statement about it had to be written twice and kept in agreement. The
concrete cost showed up in the exit-6 population: `archive-purge-evidence-divergent`
is discovered *by* that state machine, so with two owners the code could be
emitted by `prepare` while its whole remediation, its "after the first per-hash
mutation" scoping and its route all belong to `purge`. rev-8 gives the
transaction one owner. A normal mutating `prepare` observes `purge_pending` and
stops at exit 3 with the whole tree unchanged; `feature intent-archive purge
--yes` is the only command that finalizes it (§9.7.2). The trade is one extra
invocation in a rare state, against a refusal, a remediation and an exit-6
population that each have exactly one command that can produce and resolve them.
The alternative — keeping the second owner and duplicating the archive
remediation into `prepare`'s output — is recorded and rejected in §21
(PIB-525, PIB-526).

**File modes.** Created files use `0644` (the shipped default at
`internal/store/store.go:918-923`); replaced files preserve the existing
file's permission bits, matching the shipped writer's existing behavior
(`internal/store/store.go:871-876`). Directories use `0755` in the tracked tree
and `0700` in `.tpatch/local/`.

### 7.9 In-command rollback

If any CAS or rename in step 12 fails, the command rolls back **immediately**, in
reverse publication order, and every undo is itself CAS-gated (§7.6.3):

| Entry action | Undo | Guard |
|---|---|---|
| `create` | `(*os.Root).Remove` the canonical file | only if its identity equals the new image; otherwise refuse |
| `replace` | restore from the archive blob named in the journal, staged into the destination directory and renamed | only if its identity equals the new image; otherwise refuse |
| `archive_index` | restore from `index.preimage.json` | same guard |
| `status` | restore from `status.preimage.json` | same guard |

Then clear the journal and release the lock. Exit `5`, report
`outcome: "rolled-back"`, and state in one line what is true — which is
**not** "the tree is exactly as it was":

> Every canonical artifact, `status.json` and the archive index are back to
> their pre-run bytes. Archive blob files written by this attempt remain as
> orphans and are listed below; remove them with
> `tpatch feature intent-archive purge <slug> --orphans --yes`.

rev-0's rolled-back message claimed the whole tree was byte-identical while
additive blobs remained on disk. That is now a named, listed, removable
residue, and PIB-316 asserts the message enumerates every orphan blob it left
(PIB-317 asserts the count is exact).

If the rollback itself fails, or any undo CAS refuses, the journal is **kept**,
the lock is released, and the command exits `6` with a report naming the
journal path, the archive directory, the specific entry that could not be
restored, the blob or preimage file that holds its old bytes, and the §6.6
abandon route. No further automatic action is attempted (PIB-110, PIB-111,
PIB-112).

### 7.10 Crash phases — enumerated, each with its recovery outcome

The lock is process-lifetime, so in **every** row below the next invocation
acquires the lock without ceremony; there is no stale-lock analysis and no row
in which the slug stays blocked.

| Phase | Crash point | On-disk evidence | Recovery outcome |
|---|---|---|---|
| CP0 | before the lock exists | nothing | nothing to do |
| CP1 | workspace directory lock held, before journal | maybe a staging tree; no lock artifact | process death releases the descriptor; next mutator acquires the same root inode, removes the stale staging trees as lane hygiene and **proceeds with its own work** — with no journal there is nothing to recover, so nothing is terminal (§7.8 step 4) |
| CP2 | blobs written, before journal | blobs, no journal; no lock artifact | acquire the workspace authority; blobs are additive **orphans**, reported by `doctor` and removable (§9.7.3); proceed |
| CP3 | journal durable, before first rename | journal; all entries == preimage | clear journal; nothing to restore; **terminal** exit 0 `recovered`, operator re-runs |
| CP4 | after rename *k* of *n* (`0 < k < n`) | journal; first *k* == new-image, rest == preimage | undo the *k* published entries (each CAS-gated); clear; **terminal** exit 0 `recovered`, naming the *k* restored entries |
| CP5 | after the last artifact rename, before `index.json` | journal; artifacts new, index old | undo all published entries; clear; **terminal** exit 0 `recovered` |
| CP6 | after `index.json`, before `status.json` | journal; artifacts + index new, status old | undo all published entries including the index; clear; **terminal** exit 0 `recovered` |
| CP7 | after `status.json` rename, before journal clear | journal; **every** entry == new-image | **complete**: clear journal only; publish nothing, undo nothing; **terminal** exit 0 `recovered`, reporting that the prior run had already completed |
| CP8 | after journal clear, before lock release | no durable lock residue | process death releases the descriptor; the next invocation finds no journal and proceeds with its own work |
| CP9 | any of the above, plus a third party wrote one of the entries | at least one entry matches neither preimage nor new-image | **refuse** (exit 6, `recovery-divergent`), preserve every file, the journal and the archive; name them and name §6.6 |
| CP10 | `--manual` crash after the workspace directory lock, before or during the single `status.json` rename | `status.json` old or new; no journal; no lock residue | acquire; nothing to recover (single-file publication, ADR-035 D3); `FEATURES.md` may be stale and is reconverged by the next transition (§12.3.1) |
| CP11 | crash between an archive blob write and the journal write, on a **retry** of the same regeneration | journal absent; blob already present with matching bytes | acquire; nothing is pending, so nothing is terminal; the blob is reused, not rewritten (content-addressed idempotency, `internal/store/resource_publish.go:240-246`); proceed |
| CP12 | purge after a per-hash removal-pending index rename, before blob removal | pending `h`; hash-correct `h.blob` present | the next `feature intent-archive purge <slug> … --yes` **claims `h` globally** first — one CAS rewrite making every reference to `h`, in every generation, removal-pending, taken while `h.blob` is present, regular and hash-correct — re-reads and revalidates that no reference to `h` is anything but pending, then removes `h.blob` under the workspace lock, then tombstones every `h` reference, then **returns** exit 0 `recovered` without starting new work (§9.7.2). The claim is what makes the removal safe: the blob is never removed while a retained reference to `h` survives. A mutating `prepare` in this state refuses exit 3 `recovery-pending` and names that command (§7.8 step 5). A **non-crash** failure at the same point is the `archive-purge-partial` outcome on branch (a) (§9.7.2, exit 5, `resume: "pending-recovery-then-completion"`), whose reported retry drives exactly this recovery |
| CP12a | purge after `h.blob` removal, before the tombstone CAS | pending `h`; `h.blob` absent | the next `purge … --yes` CAS-publishes **every** reference to `h` — pending, already-tombstoned, and any retained reference an external write inserted after the removal — as tombstoned under the workspace lock in one rewrite, and **returns** exit 0 `recovered`; no removal is retried and none is invented, which is exactly the shipped dangling repair's shape (§9.3.1). A mutating `prepare` again refuses `recovery-pending` rather than finalizing it. The same non-crash failure here is likewise branch (a) of `archive-purge-partial`, resumed by the sanitized equivalent retry |
| CP13 | duplicate-generation rehydration after blob durability, before index rename | old tombstone plus the present blob it names | the classification is decided from the hash's **global** liveness (§9.3). If no other reference to that hash is retained or removal-pending, the blob is **unreferenced physical residue**: a retry strict-decodes X1–X10, then X11 observes the tombstone beside its present named blob and refuses exit 3 `archive-index-storage-inconsistent` with the whole tree unchanged, naming `tpatch feature intent-archive purge <slug> --orphans --yes` (§9.3.1 subcase A). After that one command removes the residue, an ordinary retry rehydrates by CAS and appends no duplicate generation. If another reference to the same hash **is** retained, the blob is live, `--orphans` excludes it, and X11 refuses the same code as the mixed tombstone/live-reference state whose one repair is the confirmed `tpatch feature intent-archive purge <slug> --blob <hash> --yes` (§9.3.1 subcase B, PIB-531). If instead another reference to the same hash is **removal-pending**, the hash is owned by a purge transaction in flight and ownership outranks both subcases: every observer other than `purge --yes` reports `recovery-pending` and routes to the owner, which claims the hash globally and finishes it (§9.3, §9.7.2, PIB-539) |
| CP14 | live workspace root deleted/recreated after a publication rename, before final verification | held old root plus a different or missing live root pathname | preserve journal/evidence; final verification exits 6 `workspace-root-replaced-after-publication`, never 0 |

CP7 is why recovery decides from **evidence, not phase**: the process died after
the semantically final act, and no marker write could have made that
distinguishable without introducing its own crash point. This is the same
reasoning the shipped land journal records
(`internal/cli/land_journal.go:11-23`) and applies to the opposite direction.

Rows: PIB-116 … PIB-123, PIB-126, PIB-291 … PIB-295, PIB-318, PIB-319,
PIB-415, PIB-443, PIB-466 … PIB-468, PIB-483 … PIB-486, PIB-531, PIB-534,
PIB-539, PIB-540.

### 7.11 Recovery: entry points, idempotency, cleanup

**Entry points.**

- **Automatic**: every mutating `prepare` for that slug **except
  `--abandon-transaction`** runs journal recovery under the lock before any
  other work, exactly as `land` does
  (`internal/cli/land_journal.go:445-482`).
- **Archive recovery**: `feature intent-archive purge --yes` — and **only**
  that command — runs the §9.7.2 pending-hash recovery, under its own
  authority and before any new archive mutation. It **claims each pending hash
  globally** before acting on it, consumes present/absent pending states
  deterministically and refuses wrong/unsafe evidence. It never
  recovers a prepare journal, and refuses `recovery-pending` if one exists
  (§9.7). A mutating `prepare` that observes a removal-pending index does
  **not** finalize it: it refuses exit 3 `recovery-pending` with zero writes
  and names this command (§7.8 step 5, PIB-525, PIB-526). A `purge` **preview**
  runs no recovery either — it takes no lock and writes nothing, and instead
  reports that recovery is required and names the `--yes` rerun of the
  operator's own selector (§9.7, PIB-515, PIB-529).
- **Operator**: `tpatch prepare <slug> --abandon-transaction --yes` (§6.6) for
  the cases automatic recovery refuses. It runs **instead of** automatic
  recovery, not after it: its branch is taken at §7.8 step 2, before the Git
  gate and before both the journal recovery and the pending-hash gate, so the
  evidence it moves is the
  evidence that was on disk when the operator looked at it (PIB-452).
- **Diagnostic**: `tpatch doctor` gains a check that *reports* pending or
  orphaned transaction state, never acts on it, and never opens or locks the
  workspace root (§12.5).
- **Nothing else.** `--check`, `next`, `cycle`, `status`, `verify`, `record`,
  `land`, `reconcile` and the phase commands neither recover nor refuse on a
  pending journal (§7.14, PIB-118 … PIB-123). `--dry-run` refuses rather than
  guessing (§6.4).

**Recovery is terminal. It never continues into the requested operation.**

A successful recovery — journal undo/clear at §7.8 step 4 of a mutating
`prepare`, or pending-hash finalization inside `feature intent-archive purge
… --yes` (§9.7.2) — ends the invocation:

| Property | Value |
|---|---|
| Exit code | `0` |
| `outcome` | `recovered` (§10.2's closed set) |
| `action` | `none` — the requested action was not performed |
| Report object | `recovery` (§10.2): `kind`, what was restored or finalized, the sanitized `retry` and `retry_cwd` |
| Advisory | `recovered-prior-transaction`, naming the restored entries or finalized hashes (§10.3) |
| What runs next | nothing in this invocation; the operator re-runs the reported retry |

Three consequences are normative:

1. **No exit-2 or exit-3 gate is evaluated after a recovery write.** Lifecycle
   state, artifact admissibility, bundle coherence, provider authority and every
   archive gate live at §7.8 step 6 and below, and a terminal recovery never
   reaches them. This is what makes §10.4's zero-write promise for exit 3
   **unqualified** rather than "zero-write except after recovery", and it is why
   PIB-469 can snapshot the whole tree for every exit-3 population without a
   post-recovery exception.
2. **Recovery failure is unchanged.** Divergent evidence, a J1–J10 bind failure
   and a refused undo CAS remain exit 6 with everything preserved; a rollback
   that itself aborted remains exit 5 per §7.9. Those paths never claimed
   zero-write and do not now.
3. **Nothing is recovered silently, and nothing auto-continues.** There is no
   flag, config key or environment variable that makes recovery fall through
   into the requested operation, and no code path that performs the requested
   operation in the same invocation as a recovery. rev-5's fall-through is
   removed rather than made optional, because an option here would mean the exit
   code's meaning depends on a flag the harness may not have set (PIB-483,
   PIB-484, PIB-487).
4. **The pending-purge transaction has exactly one owner.** `RecoverPendingPurge`
   has one call site and it is on the `feature intent-archive purge --yes`
   path. No mutating `prepare` mode, no preview and no diagnostic reaches it,
   which is what gives `archive-purge-partial` and
   `archive-purge-evidence-divergent` a single command that can both produce
   and resolve them (§7.8 step 5, §9.7.2, §10.4.1, PIB-525, PIB-526).

**What is *not* a recovery.** With no journal present, removing stale `stage-*`
directories for the slug is ordinary lane hygiene: nothing was pending, nothing
is restored, no `recovered` outcome is emitted and the invocation proceeds with
the work it was asked to do (CP1, CP2, CP11). The terminal rule fires only when
there was a pending transaction to undo/complete, or — in
`feature intent-archive purge --yes` alone — a pending archive hash to
finalize (PIB-483).

**Why not "every relevant command must recover or refuse".** That alternative
was evaluated and rejected (§8.2 alternative J2): it would put a write path
into read-only commands, add a lock acquisition to eleven shipped verbs, and
make a stale journal able to wedge `tpatch status`. The chosen model is
**pointer-free and generation-free**: a pending journal is a fact about one
slug's `.tpatch/local/` lane, it never changes the meaning of any canonical
file, and the canonical tree remains self-describing without it. Because
recovery is undo-only, a never-recovered journal leaves the tree in a state
that is *already* one of the enumerated CP-phases — mixed, but classified
truthfully by the accepted read-only check.

**Idempotency.** Recovery is a function of evidence, so running it twice is a
no-op the second time: the journal is gone. A recovery interrupted mid-undo
leaves a state that is itself a valid CP-phase and is re-recoverable (PIB-113).

**Cleanup.** Recovery removes: the journal, both preimage files, the staging
tree named in the journal, and any other `stage-*` directories under that
slug's lane. It never removes archive blobs, never removes the archive index,
never removes an `abandoned-*` evidence directory, and never touches another
slug's lane (PIB-114, PIB-115, PIB-320).

#### 7.11.1 The T2 boundary: a lost journal is unrecoverable and usually undetectable

The journal is intentionally in the gitignored local lane. `git clean -xfd`, a
fresh clone, or an out-of-band deletion can remove it while a transaction is
pending. If that happens, tpatch has lost the undo evidence. It cannot recover
old bytes by guessing from canonical artifacts; for a regeneration, any archive
blobs that remain are merely the remaining recovery material.

**There is no general journal-loss detector.** A normal sequence of
`analyze`/`define`/`explore`, or a hand-authored partial bundle, can produce the
same mixed canonical artifacts as an interrupted prepare. No journal plus mixed
canonical bytes proves neither `prepare` nor journal deletion. `doctor` must
not label that ordinary lifecycle state an interrupted transaction and must not
offer recovery for it (PIB-407).

D9 reports only evidence-bearing **current residues** — a present journal,
staging/abandoned control evidence, an orphan blob, a mixed
tombstone/live-reference hash, a corrupt blob under a retained reference, an
owned (removal-pending) hash, a corrupt archive index, or a dangling live
reference — and describes each without inferring why it exists.
A completely cleaned lane/archive after a lost journal is indistinguishable
from a workspace that never ran `prepare`; the loss is therefore an explicit,
undetectable T2 boundary. `git clean` and a fresh clone both take this ordinary
no-evidence branch; neither causes a journal-loss diagnostic (PIB-321,
PIB-322). The mandatory disclosure says this limitation, not that `doctor`
detects it (PIB-323).

**Rejected: journal in Git control state.** It would not survive a fresh clone,
would colonize Git's namespace across worktrees/submodules and `.git` files,
would require a second non-Git lane, and does not improve the held-root authority. It is not a solution to the evidence-loss boundary.

### 7.12 Concurrency matrix

| Concurrent actor | Behavior |
|---|---|
| A second mutating `prepare` or archive `purge --yes`, **any slug in the same workspace** | The single directory authority contends: exit 3, `transaction-in-progress`; the holder is unaffected (PIB-124, PIB-125, PIB-397). |
| Holder process killed | Kernel close releases the directory flock; the next invocation acquires, recovers from evidence and proceeds (PIB-396). |
| Alias path at acquisition; root rename later | An alias acquired before the rename contends on the same inode. The original discovered pathname cannot be rediscovered after a rename: missing/changed live path refuses before the window or exits 6 after it; held-root I/O does not make the moved path discoverable (PIB-413, PIB-436). |
| Root deletion/recreation | A new root can split authority; live identity mismatch refuses before the window or exits 6 with preserved evidence after it (PIB-414, PIB-415). |
| `prepare --check`, any slug; any `--dry-run`; archive `list`; **archive `purge` preview on every selector, including the pending-hash and pending-journal cases** | Never acquire the directory lock and report/plan from observed bytes. Only `purge --yes` acquires (§7.4.1, PIB-490, PIB-515, PIB-530). |
| Editor, Git, `define`, `cycle`, other local user or other machine | Not excluded. Preflight/CAS detects only changes that land before its check. A write in CAS→rename/remove's final syscall window can be overwritten or removed; this is disclosed, never called total CAS (PIB-103, PIB-421, PIB-422, PIB-430). |
| `git clean -fd` / `git clean -xfd` | Can remove untracked `.tpatch/features/**` archives as well as `.tpatch/local/**`; after external deletion archive recovery exists only from identical later archival or committed Git history (§9.6.2, PIB-426). |
| Unmount/eject or an explicitly denied/failed root filesystem | Fails closed at authority acquisition or yields the classified pre/post-publication failure; no unlocked fallback and no cross-machine guarantee. |

### 7.13 Git usage — closed, read-only, conditional and single-probe

Git is a read-only privacy dependency for the mutating local-lane gate. The
prepare Git executor has a **closed allowlist**: it may spawn only G1–G4 below,
with the stated argv; any other Git argv is a source-guard failure. This is not
a “zero Git” rule. Every G1–G4 command has `cmd.Dir` exactly the discovered
workspace root and a C locale (`LC_ALL=C`, `LANG=C`).

| # | Exact argv | Conditional count |
|---|---|---|
| G1 | `git rev-parse --is-inside-work-tree` | exactly once per **normal mutating `prepare` invocation** — `generate`, `manual`, `regenerate`; never in `--check`, `--dry-run`, `--abandon-transaction`, `intent-archive list` or `intent-archive purge` |
| G2 | `git check-ignore -q --no-index -- <lane-rel>` | once only after G1 established `true` |
| G3 | `git --literal-pathspecs ls-files -- .tpatch/local/` | once only after G1 established `true` |
| G4 | `git ls-files -- .tpatch` | once only for regenerate after G1 established `true` |

G1 returns `true` → `GitWorktree` and permits G2/G3 (and G4 only for
regenerate); a clean C-locale non-worktree result → `GitNonWorktree`, zero
G2–G4 and advisory `workspace-not-git`; any
lookup/spawn/cwd/unexpected-output/unclassified error → `GitUnverifiable` and
`local-lane-unverifiable` (exit 3) before a local-lane byte. Thus a mutating
non-regenerate worktree invocation has exactly G1+G2+G3 (three processes), a
regenerate worktree invocation exactly G1–G4 (four), and a Git-established
non-worktree exactly G1 (one). Dry-run, archive `list`, **archive `purge`** and
`--abandon-transaction` execute **zero** Git processes.

**`--abandon-transaction` is exempt, deliberately.** The gate's purpose is to
establish that the gitignored lane really is ignored *before tpatch writes new
content into it*. Abandon writes no new content: it renames existing lane
entries one directory deeper inside the same lane (§6.6 rule 2). Making the one
escape from exit 6 depend on a working `git` would convert an absent or broken
Git into a permanent block, so the gate is skipped in that mode and the
exemption is stated here as well as in §6.6 (PIB-492, PIB-493, PIB-494).

**`feature intent-archive purge` is exempt too, for a different and simpler
reason: it has no lane.** rev-6's table said G1 ran "per mutating invocation or
purge", which implied a lane gate for a command that never touches the lane.
Purge's entire write set is the **tracked** archive under
`.tpatch/features/<slug>/artifacts/intent-archive/`: a same-directory
temp→`Root.Rename` rewrite of `index.json`, and rooted removals of
`blobs/<hash>.blob`. It creates no `stage-*` tree, writes no journal, writes no
preimage and writes not one byte under `.tpatch/local/`, so G2's question — is
the gitignored lane really ignored — has no subject in that command. The
exposure question purge *does* raise is the tracked-retention one, and that is
answered by §9.6.2's disclosure and the `bundle-untracked-in-git` advisory on
the regenerate path that created the archive, not by a gate re-run at deletion
time. Purge therefore executes **zero** Git processes in both its preview and
its `--yes` form, on every selector, and a process spy asserts the zero rather
than a count (PIB-514). Nothing about the privacy boundary weakens: purge only
ever removes bytes from a tracked directory and rewrites a tracked manifest.

#### The pinned environment scrub

Every prepare-specific G1–G4 invocation runs with the following **closed,
pinned** list of variables removed from the child environment. The list is
exact — an implementation may not shorten it, and adding to it requires
amending this table:

| Variable | Why it is scrubbed |
|---|---|
| `GIT_DIR` | selects another repository's object/ref store |
| `GIT_WORK_TREE` | selects another working tree |
| `GIT_INDEX_FILE` | selects another index, changing `ls-files` answers |
| `GIT_COMMON_DIR` | selects another shared directory across worktrees |
| `GIT_CEILING_DIRECTORIES` | truncates discovery and can turn a worktree into a non-worktree |
| `GIT_OBJECT_DIRECTORY` | selects another object store |
| `GIT_ALTERNATE_OBJECT_DIRECTORIES` | adds foreign object stores |
| `GIT_DISCOVERY_ACROSS_FILESYSTEM` | changes what `rev-parse` calls a worktree |
| `GIT_PREFIX` | changes relative-pathspec interpretation |
| `GIT_CONFIG_COUNT` | enables the indexed inline-config protocol |
| `GIT_CONFIG_KEY_<n>` / `GIT_CONFIG_VALUE_<n>`, **every index** | inline config injection; each pair can rewrite `core.excludesFile`, `core.attributesFile` or any other key for this invocation only |

The `GIT_CONFIG_KEY_<n>`/`GIT_CONFIG_VALUE_<n>` rule is **not** bounded by
`GIT_CONFIG_COUNT`: the implementation removes every environment variable whose
name matches `^GIT_CONFIG_(KEY|VALUE)_[0-9]+$`, whatever the count says, so a
stale higher-indexed pair cannot survive a count change. `GIT_IMPLICIT_WORK_TREE`
and `GIT_SUPER_PREFIX` are additionally scrubbed as selection-adjacent
variables. PIB-474 asserts the exact resulting environment, including the
indexed forms and the C locale.

**Global and system ignore configuration remains intentionally available.**
`GIT_CONFIG_GLOBAL`, `GIT_CONFIG_SYSTEM`, `HOME` and `XDG_CONFIG_HOME` are
**not** scrubbed, and this is a deliberate, disclosed asymmetry rather than an
omission. G2's question is "is this lane effectively ignored **for this
operator**", and the honest answer depends on the operator's own
`core.excludesFile` — commonly `~/.config/git/ignore`. Suppressing the
operator's persistent configuration would make the gate disagree with what the
operator's own `git check-ignore` reports and would produce a false
`local-lane-not-ignored` refusal for a correctly configured workspace
(PIB-475). The scrub is therefore scoped to (a) **repository/index/object
selection** and (b) **per-invocation inline config injection**, both of which
change *which repository is answered about* rather than *what the operator
configured*. This is not a sandbox and is not described as one: a caller who
can set `GIT_CONFIG_GLOBAL` in this process's environment can also set `HOME`
and `PATH`, and no environment scrub defends against that.

**No absolute path is passed or printed.** Every lane argument in G2–G4 is
repo-relative (`.tpatch/local/…`, `.tpatch`), `cmd.Dir` carries the location
instead, and no report field, refusal message or advisory renders an absolute
path (PIB-476).

#### The central executor and its callers

`internal/gitutil/ignore.go` owns the central scrubbed executor and
`GitState`; its new state-consuming ignore/untracked helpers accept the
already-established tri-state plus a repo-relative lane path. They do not call
`IsGitAvailable`, G1 or compose/report absolute lane paths.
`internal/workflow/session_ignore.go`, `internal/rescap/scratch.go` and
**`internal/rescap/gitgate.go`** are authorized consumers/refactors of that
central gate, not duplicate privacy gates. `gitgate.go` is named explicitly
because it is where the shipped `check-ignore` / `ls-files` executor actually
lives for the resource-capture lane
(`internal/rescap/gitgate.go:45-118`); a refactor that centralized
`gitutil/ignore.go` and `session_ignore.go` while leaving `gitgate.go` spawning
its own unscrubbed Git would leave exactly the duplicate privacy gate this
decision forbids (PIB-472).

**Existing callers keep explicit compatibility wrappers**, and those wrappers
retain the callers' **current** environment, call shape, exit-code
interpretation and output behavior — including `gitgate.go`'s two documented
invocation asymmetries (`check-ignore` without `--literal-pathspecs` and the
leading-colon `./` rule, `internal/rescap/gitgate.go:1-41`). This PRD
deliberately authorizes **no** behavior change for those callers: the refactor
moves the process-spawning seam, not the semantics, and their goldens must stay
byte-identical. Two policies are therefore not hidden behind one function — the
wrapper is where the difference is stated, and PIB-473 fails if a wrapper
silently changes its caller's environment or output. If a future wave wants the
prepare scrub applied to the shipped callers, that is a separate, deliberately
authorized and separately tested change, not a side effect of this one.

The exec spy asserts the closed argv set, the pinned scrub (including the
indexed config forms), the preserved global/system ignore configuration, C
locale, relative lane arguments, the abandon-mode exemption, the purge-command
exemption and every conditional count (PIB-107,
PIB-279…PIB-283, PIB-326, PIB-408, PIB-427, PIB-438, PIB-439, PIB-472…PIB-476,
PIB-492…PIB-494, PIB-514).

Git is never used for authority, lock identity, recovery, or blob removal. It
never stages, commits, checks out, stashes or writes `.git/**`.

### 7.14 What the read-only check reports while a journal is pending

**Nothing about the journal.** This is a decision, not an omission:

1. The accepted `--check` report schema is frozen and its field set is closed
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2420-2576`). Adding a
   field would be a `schema_version` change to an accepted contract that this
   PRD is explicitly forbidden to modify.
2. `--check`'s subject is canonical artifacts. A journal is transient control
   state in a gitignored lane; it is not an artifact and has no state in the
   accepted nine-value enum.
3. What the operator actually needs — "is a prepare half-done here?" — is
   answered truthfully by `--check` anyway, because a mixed set reports as
   mixed, and by the new `doctor` check by name (§12.5).

So: `prepare --check` is byte-identical whether or not a journal exists, given
identical canonical bytes (PIB-207). The pending transaction is surfaced by
`doctor`, resolved by the next mutating `prepare` — which recovers, reports
`recovered` and stops (§7.11) — and escapable through §6.6.

## 8. Non-destructive overwrite — the architecture gate

### 8.1 The question

`--regenerate` overwrites files a human may have hand-authored. The dispatch
constraint is absolute: **it must not lose prior hand-authored bytes.** The
question this section answers is *by what mechanism*, and whether that
mechanism is a persistent representation — because selecting one fires the
architecture gate and requires an ADR before acceptance.

### 8.2 Alternatives evaluated

| # | Option | For | Against | Verdict |
|---|---|---|---|---|
| **H1** | **Refuse overwrite entirely.** No `--regenerate`; the operator deletes files by hand first. | Zero new machinery; no new object; trivially safe. | Does not satisfy the dispatch, which requires an explicit coherent overwrite route. Pushes the destructive act onto `rm`, which has no archive at all — strictly *less* safe in practice. | **Rejected.** |
| **H2** | **Rely on Git.** Refuse `--regenerate` unless `.tpatch/features/<slug>/` is tracked and clean; recovery is `git checkout`. | No new object; uses a tool the user already has; zero storage growth. | `.tpatch/` is frequently untracked — the CLI ships a dedicated detector for exactly that condition (`internal/cli/cobra.go:3405-3407`) and warns about it. A Path B agent that authored three files five minutes ago has committed none of them. The guarantee would evaporate in the single most common case it is needed. It would also turn Git from a *conditional* read-only dependency (§7.13) into a hard precondition for the one mode that destroys bytes. | **Rejected as the sole mechanism.** Retained as a *reported* advisory (§10.3). |
| **H3** | **Ephemeral rollback-only journal.** Keep preimages only for the duration of the transaction; delete them at commit. | Satisfies T1/T2 with no durable growth; no new tracked object; no ADR. | Satisfies crash-safety and *not* the actual requirement: after a **successful** `--regenerate`, the prior bytes are gone forever. The dispatch's requirement is about the success path, not the crash path. | **Rejected for `--regenerate`.** Adopted for the transaction half (§7.5) — the journal *is* this, and it is sufficient there precisely because the archive covers the success path. |
| **H4** | **Durable immutable, content-addressed intent-generation snapshots; canonical files remain the sole authority.** Replaced bytes are copied to `artifacts/intent-archive/blobs/<sha256>.blob`; an `index.json` names which artifacts each generation replaced. No reader consults the archive to determine current state. | Prior bytes survive success/crash while present and, once committed, clone and machine change. An untracked archive remains subject to external deletion including Git clean. Content addressing dedupes: regenerating to identical content writes zero new bytes. Directly reuses the shipped, reviewed resource-capture shape — immutable content-addressed set plus one atomically-rewritten pointer (`internal/store/resource_publish.go:1-9,219-285`). Canonical readers are entirely unaffected. | Creates **durable retention of bytes the operator was previously destroying** (§9.6.2), so it requires the ADR-027 D3 redaction gate (§9.6.1) and a bounded removal surface (§9.7) as parts of the selection, not as follow-ups. Grows the tracked tree (mitigated: content-addressed, so only *distinct* content costs bytes; purge bounds it). Adds one tracked object and therefore **fires the architecture gate**. | **SELECTED, with §9.6 and §9.7 as conditions of the selection.** |
| **H5** | **Pointer-based generation directories.** Canonical paths become pointers/symlinks into `generations/<id>/`. | Cheap "switch the whole bundle" semantics; history is a first-class directory. | Breaks every existing reader: `os.ReadFile` on `spec.md` now depends on link resolution, and the accepted inspector **refuses symlinks by design** (`docs/prds/PRD-artifact-validation-and-provenance.md:1768`), so the bundle would classify as `symlink-refused` — the check would call every prepared feature unsafe. Also breaks Git checkout on Windows without developer mode. | **Rejected outright.** |
| **H6** | **Sub-record on `FeatureStatus` / a dedicated manifest carrying prior content or hashes.** | Written atomically with state by the one writer (`internal/store/store.go:368-377`); the ADR-031 D1 argument. | A hash is not the bytes: it proves change, it does not recover content. Inlining content into `status.json` bloats the hottest file in the feature directory and makes every state transition rewrite it. And a *hash of an artifact* is exactly the shape §9.8 must avoid being read as a provenance claim. | **Rejected.** |

### 8.3 Selection

**H4 is selected**, with the transaction half of H3 (§7.5) retained for
crash-safety and H2 demoted to an advisory (§10.3). The selection is
**conditional**: the redaction precondition (§9.6.1), the retention disclosure
(§9.6.2) and the listing/purge surface (§9.7) are part of what is selected. An
H4 without them is a different, unacceptable design — it retains secrets
silently and forever.

### 8.4 The gate fires — ADR-035 is required

H4 is a **persistent history representation**: a new durable, tracked object
with its own wire format, its own identifiers and its own lifecycle. Per the
dispatch and per AGENTS.md's "ADR on every architecture decision" rule, this
PRD therefore creates
[`ADR-035-intent-bundle-publication-and-history.md`](../adrs/ADR-035-intent-bundle-publication-and-history.md)
as **Proposed rev-11**, keeps it in the ADR index, and requires it to be
reviewed together with this PRD. **Neither may be accepted alone.**

### 8.5 What this selection is NOT

Two things must not be inferred from it, and both are asserted mechanically:

1. **It is not a provenance representation, and it does not fire the WP-005
   provenance trigger.** That trigger fires on "selection of a persistent
   provenance representation" and on nothing else
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2966-2972`). The
   archive records *bytes that existed at some point at a path*. It records no
   author, no agent, no model, no Path A/Path B tag, and no source claim. It
   cannot answer "who wrote this", and nothing in this PRD's output claims it
   can. `prepare --check` keeps emitting the constant `provenance: unknown` for
   every artifact, unchanged (§9.8, PIB-140 … PIB-144).
2. **It is not a licence to cite ADR-034 as precedent for persistence.**
   ADR-034 D14 forbids exactly that
   (`docs/prds/PRD-artifact-validation-and-provenance.md:2957-2961`). ADR-034 is
   reused in this PRD for one thing only: the rooted, race-safe **read**
   boundary. Every write path in this document is governed by ADR-035
   (PIB-145).

## 9. The intent archive

Normative source: ADR-035 D8–D16. This section states the product-visible
contract.

### 9.1 Layout

```text
.tpatch/features/<slug>/artifacts/intent-archive/
├── index.json                     ← tracked manifest, canonical JSON, atomically rewritten
└── blobs/
    └── <64 lowercase hex>.blob    ← immutable, content-addressed prior bytes
```

The archive lives at tracked paths — but "tracked" is a property of the
operator's repository, not of this design, and §9.6.2 states exactly what
follows from that. Putting the archive in `.tpatch/local/` instead was rejected
because a recovery guarantee that vanishes on a fresh clone is not a recovery
guarantee (ADR-035 D8).

### 9.2 Blobs

- Name: the lowercase-hex SHA-256 of the file's exact bytes, plus `.blob`. The
  hash is over the **raw bytes**, not a canonicalization.
- Immutable **in content**: an existing blob whose bytes already equal the
  content is **never rewritten**; the publication simply reuses it.
  Content-addressed dedupe means regenerating to previously-seen content costs
  zero new bytes — the shipped idempotency argument at
  `internal/store/resource_publish.go:240-246`. Immutability means *no
  in-place modification*; it does **not** mean undeletable (§9.7).
- An existing blob file whose bytes **differ** from the content that hashes to
  its name is a corruption and refuses the invocation
  (`archive-blob-corrupt`, exit 3) rather than being overwritten — the shipped
  `batch-file-corrupt` disposition (`internal/store/resource_publish.go:198-202`).
- Written and fsynced before the journal is finalized, so a preimage is durable
  before it can be needed (§7.8 step 10).
- A blob written by an attempt that then crashed or refused is an **orphan**.
  Orphans are reported and removable (§9.7.3) — rev-0 called them "normal and
  permanent", which turned every failed attempt into unbounded, undeletable
  tracked growth.

#### 9.2.1 Archive semantics per mode — unambiguous, total

| Mode | Has `replace` entries? | Blobs written | Index entry appended | Archive directory created |
|---|---|---|---|---|
| `check` | — (read-only) | none | none | never |
| `generate` (default) | **never** (§6.1.3) | none | none | never (PIB-255, PIB-256) |
| `manual` | never (single-file publication) | none | none | never |
| `regenerate` without any present artifact | no (all `create`) | none | none | never (PIB-330) |
| `regenerate` with ≥1 present artifact | yes | one per distinct replaced content, except an already-retained duplicate | append one new generation, no-op for an already-retained duplicate, or **rehydrate** matching tombstones (§9.3) | on demand, lazily |
| `abandon` | — (no canonical write) | none | none | never |

There is exactly one route that ever archives, and it is the one the operator
typed `--regenerate` to reach.

### 9.3 `index.json`

```json
{
  "schema_version": 1,
  "feature": "fix-model-id-translation",
  "generations": [
    {
      "generation_id": "3f0c…",
      "mode": "regenerate",
      "replaced": [
        { "artifact_id": "analysis", "path": "analysis.md", "content_sha256": "a91e…", "blob": "a91e…", "size_bytes": 4211, "purged": false, "purge_pending": false },
        { "artifact_id": "spec", "path": "spec.md", "content_sha256": "b12d…", "blob": "", "size_bytes": 1902, "purged": true, "purge_pending": false }
      ]
    }
  ]
}
```

`content_sha256` is immutable identity, always the raw-byte SHA-256 that named
the original blob. `blob` is a storage locator. Storage has three closed wire
states: retained (`blob == content_sha256`, `purged == false`,
`purge_pending == false`), removal-pending (the same locator with
`purge_pending == true`), and tombstoned (`blob == ""`, `purged == true`,
`purge_pending == false`). A pending record is intentionally a durable
per-hash transition, not a claim that a removal has not happened:

**Storage observation is global by content hash, never per reference.** A
reference's storage state is a claim about `content_sha256`, and the same hash
may be referenced by several generations, so no observation about
`blobs/<h>.blob` can be classified from one reference in isolation. Two derived
predicates are used everywhere below and are computed over **every** reference
to `h` in **every** generation:

- `h` is **live** when at least one reference to it is retained or
  removal-pending;
- `h` is **unreferenced** when no reference to it is retained or
  removal-pending — every reference to it is tombstoned, or the index holds no
  reference to it at all;
- `h` is **owned** when at least one reference to it is **removal-pending**.

An **orphan** is a regular file `blobs/<h>.blob` whose hash is unreferenced by
that definition, whether the index mentions `h` only in tombstones or never
mentions it (§9.7.1, §9.7.3). A live hash is never an orphan, and `--orphans`
never selects its blob.

**Global pending ownership — the invariant the whole purge machine rests on.**
*If any reference to `h` is removal-pending, the purge transaction owns `h`
globally.* Ownership is a property of the **hash**, not of the reference that
carries the pending flag, and it has three consequences that every later section
derives from rather than restating:

1. **One transaction decides `h`'s next state.** No other command may remove
   `h.blob`, tombstone a reference to `h`, un-tombstone one, or select `h` for
   new work. `RecoverPendingPurge` — whose single call site is
   `feature intent-archive purge --yes` (§7.11) — is the only code that
   advances an owned hash.
2. **Ownership outranks every other classification of the same hash.** A
   retained, tombstoned or corrupt observation on an owned hash is **not**
   reported as dangling, residue, mixed or corrupt: it is reported as pending
   and routed to the owner. An ordinary mutating `prepare` refuses exit 3
   `recovery-pending` (§7.8 step 5); a `purge` preview reports
   `recovery-required` and takes no lock (§9.7); `list` renders the hash's
   pending state (`pending-remove` or `pending-finalize`) for **every**
   reference to it and exits 0, because an in-flight transaction is a
   recoverable state rather than an untruthful index; `doctor` reports the same
   without acting. Ordinary commands see pending and route to purge — that is
   the whole rule (PIB-539, PIB-541).
3. **The blob is removed only when *every* reference to `h` is
   removal-pending.** §9.7.2's recovery therefore *claims* `h` globally —
   CAS-publishing **every** reference to `h`, in every generation and in every
   wire state, to removal-pending in one index rewrite while `h.blob` is still
   present, regular and hash-correct — then re-reads and revalidates, and only
   then removes. rev-9 acted per reference
   and so could remove `h.blob` while another generation's reference to `h`
   stayed retained, manufacturing the dangling reference the design exists to
   prevent; rev-10 removed that composition instead of documenting it, and
   rev-11 closes the remaining hole in it (§9.7.2,
   §7.10 CP12/CP12a, PIB-539, PIB-540, PIB-545).

**The claim's domain is every reference to `h`, and no same-hash reference is
exempt from it.** rev-10 said the claim swept "every reference" in one place and
still described an already-tombstoned reference as a "deterministic no-op" in
another. Those two sentences cannot both hold: consequence 3 gates the removal
on *every* reference being removal-pending, so a reference the claim declines to
touch is a reference that blocks the removal forever, and an index holding one
tombstoned and one retained reference to `h` would deadlock the very machine
that exists to unblock it. **Rev-11 removes the exemption rather than the
gate**: the claim CAS sets retained references *and* already-tombstoned
references to removal-pending, in one rewrite, and the no-op language is
withdrawn wherever it appeared (§9.7.1, §9.7.2, §9.3.1 subcase B, PIB-532,
PIB-545).

Claiming an already-tombstoned reference changes nothing about where that
reference ends: it passes through removal-pending back to tombstoned, which is
where it already was. What it changes is the hash's **wire uniformity before any
byte is removed**, which is exactly what consequence 3 needs to be checkable.
The alternative — leaving the tombstone untouched and weakening the gate to
"every *non-tombstoned* reference is pending" — was rejected, because the gate
would then have to reason about which references it is allowed to ignore at the
moment of removal, and that is the per-reference reasoning rev-10 removed (§21).

Because all references to an owned hash are normalized to one wire state before
anything is removed, mixed wire states across the references to one hash are a
**transient** condition of a transaction in flight, never a steady state. They
are therefore not an X7 failure (X7 is per-reference, §9.3.1) and not silently
tolerated either: the observer either owns the hash and normalizes it, or routes
to the owner.

**The classification map below is total, and its precedence is fixed.** A
reference's disposition is decided from four facts — its own wire state, the one
blob observation the hash's file supports (`absent`, `present, regular and
hash-correct`, or `present but non-regular or hash-wrong`), whether the hash is
**owned**, and whether it is **live** — in this order, so no triple resolves to
zero routes or to two:

1. **Ownership first.** If `h` is owned, every reader but
   `feature intent-archive purge --yes` reports pending and routes to the owner,
   whatever the reference's own wire state or the blob's condition says.
2. **Unidentifiable bytes next.** If `h` is not owned and its file is present
   but non-regular or hash-wrong, the observation is `archive-blob-corrupt`
   while any reference to `h` is retained, and `--orphans`' per-blob identity
   refusal (the same code) while the hash is unreferenced. A corrupt file is
   never reclassified as residue, as mixed or as dangling, and the corrupt
   observation dominates the mixed one when the same file carries both.
3. **Liveness last.** Only for a present, regular, hash-correct file does the
   tombstoned-reference split of §9.3.1 apply — unreferenced ⇒ residue, live ⇒
   mixed.

| Wire state | Blob observation | Required next action |
|---|---|---|
| retained | present, regular and hash-correct, `h` **not owned** | ordinary live storage |
| retained | present, regular and hash-correct, `h` **owned** | the reference is inside the purge transaction. Not an inconsistency: the recovery's global claim normalizes it to removal-pending before any removal, and every other command routes to the owner with `recovery-pending` (§9.7.2) |
| retained | **absent**, `h` **not owned** | **dangling live reference.** X11 refuses exit 3 `archive-blob-dangling`, writing nothing, and names the one repair: the confirmed `tpatch feature intent-archive purge <slug> --blob <hash> --yes`, which tombstones every reference to `h` after confirming the blob is still absent and removes nothing (§9.3.1, §9.7.3) |
| retained | **absent**, `h` **owned** | the transaction in flight owns it. Not classified as dangling: `RecoverPendingPurge`'s absent-blob path claims and tombstones **every** reference to `h`, this one included, and every other command routes to the owner with `recovery-pending` (§9.7.2 step 5) |
| retained | present, **non-regular or hash-wrong**, `h` **not owned** | **corrupt live storage.** The index says these bytes are the recovery material for a live reference and they are not. X11 refuses exit 3 `archive-blob-corrupt`, writing nothing, and names the one route: §9.3.1's type-total removal of the managed blob path followed by the confirmed `tpatch feature intent-archive purge <slug> --blob <hash> --yes` — or restore the exact correct blob and retry (§9.3.1) |
| retained | present, **non-regular or hash-wrong**, `h` **owned** | the transaction in flight owns it: every reader but `purge --yes` routes with `recovery-pending`, and `purge --yes` finds the claim's evidence check unsatisfied and refuses exit 6 `archive-purge-evidence-divergent` with the evidence preserved (§9.7.2 step 6) |
| removal-pending | present, regular and hash-correct | claim `h` globally — every reference to it, retained and tombstoned alike — then remove it, then CAS-publish the tombstone for every reference to `h` |
| removal-pending | absent | CAS-publish the tombstone for every reference to `h`; removal already occurred |
| removal-pending | present, non-regular or hash-wrong | refuse exit 6 `archive-purge-evidence-divergent` and preserve evidence (§9.7.2 step 6) |
| tombstoned | absent, `h` **not owned** | ordinary purged storage |
| tombstoned | absent, `h` **owned** | the transaction in flight owns the hash; this reference is already at its end state and is nonetheless claimed and re-tombstoned with the rest, so the hash's wire state is uniform at every step (§9.7.2 steps 1 and 5) |
| tombstoned | present, and `h` is **owned** | the transaction in flight owns it; the recovery's global claim sweeps this reference into removal-pending and tombstones it again at the end. Every other command routes to the owner with `recovery-pending` (§9.7.2) |
| tombstoned | present, regular and hash-correct, and `h` is **unreferenced** | **unreferenced physical residue.** Nothing references those bytes and no pending reference exists, so no purge transaction is in flight and nothing can be finalized. X11 refuses exit 3 `archive-index-storage-inconsistent`, writing nothing, and names the one repair: `tpatch feature intent-archive purge <slug> --orphans --yes` (§9.3.1, §9.7.3) |
| tombstoned | present, regular and hash-correct, and `h` is **live but not owned** — some other reference to it is retained | **mixed tombstone/live-reference global-availability inconsistency.** The blob is live storage for the retained reference, so it is **not** an orphan and `--orphans` must exclude it; the tombstone nevertheless claims those bytes are gone. X11 refuses exit 3 `archive-index-storage-inconsistent`, writing nothing, and names the one repair: `tpatch feature intent-archive purge <slug> --blob <hash> --yes` (§9.3.1, §9.7.3) |
| tombstoned | present, **non-regular or hash-wrong**, and `h` is **unreferenced** | not an orphan and not residue: the file is unidentifiable, so `--orphans --yes` refuses it at its per-blob identity check. X11 refuses exit 3 `archive-blob-corrupt`, writing nothing, and names §9.3.1's type-total removal of the managed blob path, after which the hash is simply unreferenced with no file and nothing further is required (§9.7.2, §9.7.3) |
| tombstoned | present, **non-regular or hash-wrong**, and `h` is **live but not owned** | the corrupt observation dominates the mixed one, because both describe the same unidentifiable file and only one of them has a route that can act on it: X11 refuses exit 3 `archive-blob-corrupt` and names §9.3.1's type-total removal followed by the confirmed `--blob <hash> --yes`, which tombstones the retained reference and leaves the already-tombstoned one where it is (§9.3.1) |

Thus an otherwise-valid pending record remains recoverable after its first
physical removal, and a tombstone is permitted only when
`blobs/<content_sha256>.blob` is absent. The immutable digest/body remains
available to validate the generation in every state.
The canonical generation body is `{feature, mode, replaced[]}` sorted by
`artifact_id`, where every replaced item contains exactly
`artifact_id,path,content_sha256,size_bytes`; it excludes `blob`, `purged` and
`purge_pending`.
`generation_id` is the SHA-256 of that immutable body. No map or wall-clock is
permitted; fixed struct order applies.

A newly generated body with an existing `generation_id` is not blindly a no-op.
The implementation first compares its immutable canonical body to the stored
one. A mismatch is `archive-generation-id-collision`. An equal body has two
deterministic cases:

1. no matching entry is tombstoned: duplicate no-op, no blob write and no index
   append; or
2. one or more matching entries are tombstoned or removal-pending: scan their
   source bytes under the redaction gate, write/reuse every required
   content-addressed blob, then CAS-publish one rooted index rewrite that sets
   **every reference with that content hash**, in every generation, to retained.
   This is **rehydration**, not an appended generation; it leaves no
   duplicate-id no-op, orphan, or false tombstone.

Storage availability is global by `content_sha256`. Rehydrating hash `h` first
scans/validates the exact available content, writes or reuses `h.blob`, then
CAS-publishes a **single** index rewrite that changes every **tombstoned or
removal-pending** reference to `h`, in every generation, to retained. A
**dangling retained** reference (wire says retained, `blobs/h.blob` absent) is
deliberately *not* in that set: X11 refuses every ordinary archive or canonical
mutation while one exists, so no regeneration can reach this path to repair it,
and §9.7.3's confirmed purge is its only shipped repair. Once that purge has
tombstoned the references, an ordinary later regeneration that reproduces the
same generation rehydrates them through exactly this path. No tombstone may
claim absence while `blobs/h.blob` exists; where one does, the repair depends on
whether `h` is owned, live or unreferenced. If `h` is **owned** — some
reference to it is removal-pending — the transaction in flight owns the repair
and every other command routes to it (§9.7.2). If `h` is **unreferenced**, the
blob is unreferenced physical residue and X11 refuses until
`purge --orphans --yes` removes it. If `h` is **live but not owned** — some
other reference to it is still retained — the blob is that reference's live
storage, `--orphans` must
exclude it, and X11 refuses until the confirmed `purge --blob <h> --yes` makes
the index globally truthful (§9.3.1, §9.7.3). A later purge
evaluates the current global live-reference count. Repeated purge→rehydrate
cycles retain stable ids and deterministic index order. Redaction is still
checked before rehydration writes a blob; a refusal leaves storage untouched.
Blob durability precedes the rooted index rename, so a crash before it leaves
at most an **unreferenced** blob — CP13's residue, removed by
`purge --orphans --yes` — and never a live reference to a missing blob. Where
the crashed rehydration's hash was **also** referenced by a retained entry, the
blob is live rather than residue and the mixed-state repair above applies
instead; where it was referenced by a **removal-pending** entry, the hash is
owned and the pending transaction's own recovery applies (§7.10 CP13, §9.7.2).

#### 9.3.1 Strict decoding — the index is never guessed at

Every index reader — publication, rehydration, purge, list, doctor — strictly
decodes one JSON value plus EOF. Failure writes nothing.

| # | Bind | Failure code | Exit |
|---|---|---|---|
| X1 | exactly one JSON value followed by EOF | `archive-index-corrupt` | 3 |
| X2 | no unknown field at any nesting level | `archive-index-corrupt` | 3 |
| X3 | schema version equals this build; a higher version says upgrade | `archive-index-version-unsupported` | 3 |
| X4 | feature equals the directory slug | `archive-index-foreign` | 3 |
| X5 | `generations` and `replaced` are non-null arrays | `archive-index-corrupt` | 3 |
| X6 | generation ids are unique lowercase SHA-256 hex | `archive-index-corrupt` | 3 |
| X7 | every `content_sha256` is lowercase SHA-256 hex; **each reference's own** wire state is exactly retained, removal-pending or tombstoned as §9.3 defines. X7 is deliberately **per reference**: references to one hash that disagree are a transient of a purge transaction in flight, not a wire-shape error, and are decided globally at X11 — a stricter X7 would refuse the exact index the recovery is meant to normalize, bricking the archive it is protecting | `archive-index-corrupt` | 3 |
| X8 | every path is relative and contained in its feature directory | `archive-index-path-escape` | 3 |
| X9 | artifact ids are the closed four-value set and unique per generation | `archive-index-corrupt` | 3 |
| X10 | recomputing `generation_id` from the immutable pre-purge body (including `content_sha256`) reproduces its recorded value, whether or not entries are tombstoned | `archive-index-generation-mismatch` | 3 |
| X11 | over the **whole index**, never over a selection, and before every mutation a **new selector** would perform: retained references name a present regular hash-correct blob; pending references have either that blob (removal remains) or no blob (tombstone finalization remains); a tombstone has no blob **unless** the hash is owned or still live, which is itself a routing or a refusal. Completing an already-pending purge transaction is the one explicit exception to the "before every mutation" ordering, and it is stated in §9.7.2 | `archive-blob-dangling` for a missing retained blob; `archive-blob-corrupt` for a present but non-regular or hash-wrong blob under a **retained** reference, and for the same file where the hash is unreferenced or only tombstone-referenced, because unidentifiable bytes outrank the residue and mixed classifications of the same file; `archive-index-storage-inconsistent` for an unsafe/wrong **pending** blob, and — as two separate, separately routed sub-populations — for a **tombstone whose named regular hash-correct blob is present**, split by whether the hash is globally **unreferenced** (physical residue) or still **live** (mixed tombstone/live-reference). An **owned** hash (any reference removal-pending) is not classified here at all: ownership outranks, and every observer other than `purge --yes` routes to `recovery-pending` (§9.3) | 3 |

Strict JSON binds X1–X10 validate wire shape and immutable identity. X11 is a
separate storage-state validation over the filesystem. A missing **retained**
blob is a repairable dangling live reference: ordinary archive/canonical
mutations refuse, `list` renders it as `dangling`, and doctor names
`archive-blob-dangling`.

**X11's scope is the whole index, and it does not depend on the selector.**
Before every archive mutation that a **new selector** would perform — every
mutating `prepare` mode and every `purge --yes` on every selector — the
observation is computed over every
reference in every generation and over the blob directory, not over the subset
the requested selector would touch. The consequences are stated once here and
derived everywhere else:

- **A global inconsistency refuses an unrelated selector, zero-write.** An
  archive holding a mixed tombstone/live-reference hash `h₂` refuses
  `purge --orphans --yes`, `purge --blob <h₃> --yes`,
  `purge --generation <id> --yes` and `purge --all --yes` alike, at exit 3 with
  the whole tree byte-identical, naming `h₂`'s owning repair — even when the
  selector is provably disjoint from `h₂` (§9.7.1, §9.7.2, PIB-542).
- **No partial cleanup proceeds around it.** The command does not remove the
  orphans it *could* have removed, or purge the hashes it *could* have purged,
  and then report the inconsistency as a leftover. Acting on part of an index
  that is lying about another part is how a half-repaired archive becomes an
  archive nobody can reason about; fail-closed is chosen precisely so the
  operator's next observation is the same one this run refused on.
- **The admitted repairs are scoped to the repair *class* they repair, and are
  admitted only after global classification.** §9.7.2's three admissions are
  not selector exemptions from X11; each is the one shipped repair for one
  observed **class** of state, admitted past that class's own refusal after the
  strict X1–X10 decode, and admitted only when the confirmed selection **covers
  every instance of that class in the whole index**. `--orphans --yes` is
  admitted past globally unreferenced tombstone-beside-blob residue and
  **nothing else** — not past a mixed hash, a dangling retained reference or a
  corrupt blob — and if the archive holds any of those as well, `--orphans
  --yes` refuses zero-write like every other selector (§9.7.2, PIB-542,
  PIB-543, PIB-549).
- **A class may have many instances, and covering all of them is admitted.**
  Rev-10 admitted a repair only when its observation was "the *sole*
  inconsistency the global scan found", which bricked an archive holding two
  unreferenced residues or two dangling hashes: each instance individually
  disqualified the other, so no selector was ever admitted and the archive had
  no shipped repair at all. **Rev-11 replaces "sole inconsistency" with "sole
  repair class, fully covered."** `--orphans --yes` repairs *all* unreferenced
  residues in one invocation; a repeated `--blob <h₁> --blob <h₂> --yes`
  selection — the grammar takes repeats (§5.1) — repairs all instances of one
  same-class hash population, and `--all --yes` does so where it is applicable
  to that class. Mixed classes remain a zero-write refusal that renders each
  class's own route (§9.7.2, PIB-548, PIB-549).
- **The refusal always names the owning repair, and it is the repair for the
  observation that refused** — not for the selector the operator typed. Where
  more than one distinct inconsistency is present, every one of them is
  rendered with its own repair, and the exit is 3 (§9.7, PIB-541).

**The one exception is completing a purge transaction that is already
pending.** "Before every archive mutation" is a statement about mutations a
**new selector** requests. An index that already holds a removal-pending
reference is not a new request: it is an unfinished transaction whose first
mutation has already landed, whose hash is owned (§9.3), and whose completion is
the only way any global observation of that hash can become stable. So
`RecoverPendingPurge` runs **first**, before the global X11 scan of the
preflight, and it may mutate and finalize **its own owned hash** while an
unrelated inconsistency `h₂` is present elsewhere in the index. It is terminal:
it returns exit 0 `recovered` and never proceeds into the new selector. The
operator's rerun is then an ordinary new-selector invocation, it performs the
full global X11 scan, and it may refuse on `h₂` at exit 3 zero-write naming
`h₂`'s own repair. Ordering the two the other way would make an unrelated
residue permanently block the recovery of a hash whose bytes are already half
removed — a global check bricking the transaction it was added to protect
(§9.7, §9.7.2, §10.5 step 22, PIB-546). A **normal mutating `prepare`** in the
same state still refuses exit 3 `recovery-pending` and finalizes nothing; the
exception belongs to the transaction's one owner, not to every command
(§7.8 step 5, PIB-525, PIB-526).

This is fail-closed by choice, and the alternative — validating only the
selected references — is recorded and rejected in §21.

**There is exactly one shipped repair, and it is a command an operator can
type.** rev-4 offered two — the confirmed purge and "exact-content
rehydration" — but the second was unreachable: X11 refuses *before* any
regeneration could write the replacement blob, so no ordinary run could ever
supply the exact bytes. The reachable repair is therefore the only one this PRD
names:

```text
tpatch feature intent-archive purge <slug> --blob <hash> --yes
```

It tombstones **every** reference to that hash, in every generation, after
confirming the blob is still absent, and performs **no** removal because the
file is already gone (a removal spy records zero removals for that hash). An
applicable confirmed `--all --yes` has the same effect for every selected hash.
After that repair the archive is consistent again, ordinary mutation proceeds,
and a later regeneration that reproduces the tombstoned generation may use the
already-defined global rehydration path of §9.3 (PIB-457, PIB-458, PIB-459,
PIB-460).

A wrong/non-regular blob under a **pending** reference remains an
evidence-preserving refusal, never a guessed repair.

**A retained reference whose blob is present but corrupt is
`archive-blob-corrupt`, and it has one executable route.** The dangling case
above is the *absent* blob. The neighbouring observation — the file exists at
`blobs/<h>.blob` but is **non-regular** (a symlink, directory, device or FIFO)
or its bytes do **not** hash to `h` — is not dangling and is not residue: a
retained reference is claiming those bytes are the recovery material for an
artifact, and they are not. It maps to exit 3 `archive-blob-corrupt`, and rev-10
pins its behavior so it cannot be discovered later as an unclassified brick:

- **What tpatch does on discovery.** Every ordinary archive or canonical
  mutation refuses exit 3 `archive-blob-corrupt`, **writing nothing** — no
  removal, no index rewrite, no pending record — including `--manual`, which
  writes no archive at all (§6.2). `list` exits **3** and renders the storage
  state as `corrupt`, naming the repo-relative `blobs/<h>.blob` path and the
  generation ids that reference `h`; `doctor` reports the same classification
  without acting. tpatch will not delete or overwrite bytes it cannot identify,
  and it will not tombstone a live reference on the strength of a file it
  cannot validate — either act would destroy evidence the operator has not seen
  yet.
- **What repairs it, exactly.** One repo-relative procedure, named character for
  character by the refusal, by `list` and by `doctor`, run from the workspace
  root — one operator-executed destructive step, then one tpatch command emitted
  under the cluster's single retry heading exactly as §9.7.2's divergence
  procedure does:

  ```text
  1. this permanently deletes whatever object is at the managed blob path,
     including a directory and everything under it. There is no undo:
       rm -rf -- .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob
  Run this again from the same workspace root:
    tpatch feature intent-archive purge <slug> --blob <hash> --yes
  ```

  After step 1 the blob is **absent**, so every retained reference to `h` is now
  a dangling reference — and the command under the heading is exactly the
  shipped dangling repair of
  §9.7.3, already admitted past X11 for that observation. It tombstones every
  reference to `h`, in every generation, after confirming the blob is still
  absent, and performs **no** removal. The route is therefore **total**: it ends
  in an admitted, already-defined repair rather than in a second special case,
  and PIB-543 asserts the composition end to end. The `rm -rf --` line is an
  operator-executed procedure step, not a retry, and keeps its own numbering;
  the tpatch command is a retry and carries the column-0 heading and
  `retry_cwd: "workspace-root"` like every other (§9.7.2, §10.7, PIB-498).
- **Why the removal step is `rm -rf --` and why no preservation copy is
  promised.** rev-10 printed `cp <managed blob path> <a path outside .tpatch/>`
  followed by plain `rm <managed blob path>`, and told the operator both would
  work for *any* of the observed object kinds. Neither does. Against a
  **directory** — one of the three kinds PIB-543 fixtures — `cp` without `-R`
  fails and `rm` without `-r` fails, so the printed procedure is unexecutable on
  the exact fixture it was written for; against a **symlink** `cp` silently
  copies the *target's* bytes rather than the link, which is a different object
  than the one that refused; and against a **FIFO or device node** `cp` either
  blocks on open or copies nothing meaningful. A procedure that is correct only
  for the regular-file subcase is not a route for a code whose population
  includes the others. Rev-11 therefore ships **one type-total destructive
  step** and withdraws the copy promise:
  - `rm -rf -- <the validated repo-relative managed blob path>` removes a
    regular file, a symlink (the link itself, never its target), a directory
    and everything beneath it, a FIFO and a device node, on both supported
    platforms (Linux and Darwin). It is printed **only** after the explicit
    destructive warning above, on its own line, with the `--` terminator so a
    path that begins with `-` cannot become an option, and with the exact
    single managed path and **no wildcard** — the path is the one X11 already
    validated as contained in the feature's archive, so the command's blast
    radius is exactly the object that refused.
  - **If the operator wants those bytes, they must stop here and preserve the
    object with tooling appropriate to its kind** — `cp -R` for a directory,
    `cp -P`/`readlink` for a symlink, `git show` for a version-controlled
    original — and tpatch says exactly that instead of printing a copy command
    that is wrong for four of the five kinds. No generic preservation command is
    emitted, because a wrong one is worse than none: it either fails loudly
    after the operator has committed to the procedure, or succeeds while
    copying something other than the object they were told they were keeping.
- **Or restore the exact correct blob and retry.** If the operator still has the
  original bytes — from Git history or a backup — writing a regular file at
  `blobs/<hash>.blob` whose bytes
  hash to `<hash>` satisfies X11, and the next ordinary invocation proceeds with
  the reference still retained and nothing purged. tpatch does not perform this
  for the operator, because it cannot know which bytes are correct; it names the
  option so the destructive route is not presented as the only one.
- **What it costs, stated in the message.** The purge route destroys this hash's
  archived recovery material: after it, the generation is tombstoned and there
  is nothing to rehydrate from until identical content is archived again. If
  that blob was ever committed, it is still in this repository's Git history;
  removing it from history is not something tpatch does (§9.6.2). The restore
  route costs nothing but requires the operator to supply bytes.
- **`--orphans --yes` never touches it.** `h` is live — a retained reference
  names it — so its blob is not in the orphan set whatever state the file is in,
  and the per-orphan identity check of §9.7.2 is a check over the globally
  unreferenced set only. A corrupt file whose hash is *unreferenced* keeps the
  separate `archive-blob-corrupt` refusal §9.3.1 subcase A and §9.7.3 already
  give it (PIB-543).

**A tombstone whose named blob is present is never divergence, and which repair
applies is decided globally over the content hash.** rev-7 filed this
observation with the pending-hash divergence population of §9.7.2 and offered it
that population's escape: report the pending hash, remove the divergent managed
blob path, rerun, and let the pending+absent case finalize the tombstone. That
is unexecutable here, because there **is** no pending reference. The reference is
already tombstoned: the index claims nothing about those bytes *through that
reference*, no purge transaction is in flight, no hash can be named as pending,
and the rerun the escape points at would find nothing to finalize.

rev-8 corrected the classification but evaluated it **per reference**, which is
the remaining error rev-9 closes. `content_sha256` is global: the same hash may
be referenced by several generations, and one of them being tombstoned says
nothing about whether the bytes are still live for another. Calling every
tombstone-beside-a-blob an orphan would let `--orphans --yes` delete a blob that
a **retained** reference still names, converting live storage into a dangling
reference — destroying recoverable bytes through the one selector whose whole
promise is that it removes nothing anybody references. X11 therefore computes
`h`'s global liveness (§9.3) before classifying — after ownership has been ruled
out, since an owned hash is routed to its purge transaction rather than
classified here at all — and the observation splits into
two sub-populations with two different repairs:

**Subcase A — `h` is unreferenced: unreferenced physical residue.**

- **What it is.** `blobs/<h>.blob` exists as a regular file while **every**
  reference to `h`, in every generation, is tombstoned — no reference to `h` is
  retained and none is removal-pending. Nothing references it. It is the same
  object §9.7.3 calls an **orphan**, observed through the index rather than
  through a directory scan — CP13's window (blob durable, rehydration index
  rename not yet landed) produces it, and so does an external recreation of a
  blob after a completed purge.
- **What tpatch does on discovery.** X11 refuses exit 3
  `archive-index-storage-inconsistent`, **writing nothing** — no removal, no
  index rewrite, no pending record — for every ordinary archive or canonical
  mutation. `list` exits **0** and renders the storage state as `orphan`;
  `doctor` reports it as orphan evidence. Neither acts on it. tpatch does not
  delete a file the operator never asked it to delete, and it does not silently
  un-tombstone a reference to adopt bytes it did not put there.
- **What repairs it, exactly.** One command, named literally, character for
  character, by the refusal, by `list` and by `doctor`:

  ```text
  tpatch feature intent-archive purge <slug> --orphans --yes
  ```

  run from the workspace root. `--orphans` is **explicitly admitted past this
  X11 refusal** — after the strict X1–X10 wire decode has passed, so the index
  is known to be well-formed before anything is removed — because that
  invocation is this state's one shipped repair, exactly as the confirmed
  `--blob <h> --yes` is the one shipped repair for a dangling retained
  reference. It validates the file as a regular blob whose bytes hash to its
  name, removes it, rewrites **no** index (§9.7.1), and leaves the tombstone
  saying what was already true. A non-regular or hash-wrong file at that path
  is not an orphan and keeps its existing `archive-blob-corrupt` exit-3
  refusal, which is likewise zero-write and preserves the evidence.
- **What reports it.** `purge --orphans` **without** `--yes` reports the residue
  — the repo-relative `blobs/<h>.blob` path and the fact that every reference to
  `h` is tombstoned — takes no lock, writes nothing, and names the `--yes`
  rerun.

**Subcase B — `h` is live: a mixed tombstone/live-reference global-availability
inconsistency.**

- **What it is.** `blobs/<h>.blob` exists as a regular file, at least one
  reference to `h` is tombstoned, and at least one **other** reference to `h`,
  in the same or another generation, is **retained** — and no reference to `h`
  is removal-pending, because a pending reference would make the hash *owned*
  and route the whole observation to the purge transaction instead (§9.3). The
  blob is
  that live reference's storage. The index is not describing residue; it is
  making two contradictory global claims about the same bytes — one reference
  says they are gone forever, another says they are here. Ordinary operation
  cannot produce it: purge selects **every** reference to a hash (§9.7.1),
  `--generation` refuses `archive-blob-shared` rather than splitting a hash, and
  rehydration un-tombstones every reference to a hash in one rewrite, so all
  references to `h` normally share one wire state. An external index edit, a
  partial hand-merge or a restored older `index.json` produces it.
- **What tpatch does on discovery.** X11 refuses exit 3
  `archive-index-storage-inconsistent`, **writing nothing**, for every ordinary
  archive or canonical mutation. The blob is **live data**: it is not an orphan,
  `--orphans` never selects it, and no `--orphans --yes` invocation may remove
  it under any circumstances. `list` exits **3** — the same exit its other
  untrustworthy-index population takes — renders the repo-relative
  `blobs/<h>.blob` path together with the generation ids that tombstone `h` and
  the generation ids that still reference it live, and names the repair. Nothing
  is removed, rewritten or un-tombstoned; `doctor` reports the same
  classification without acting.
- **What repairs it, exactly.** One command, named literally, character for
  character, by the refusal, by `list` and by `doctor`:

  ```text
  tpatch feature intent-archive purge <slug> --blob <hash> --yes
  ```

  run from the workspace root. It is the ordinary confirmed global-hash purge,
  not a special mode: `--blob h` selects **every** reference to `h` (§9.7.1),
  the claim CAS sets **all** of them — the retained references and the
  already-tombstoned ones alike — to removal-pending in one rewrite, and the
  normal per-hash machine then removes the blob and tombstones every pending
  reference (§9.7.2). The already-tombstoned reference is **not** skipped as a
  no-op: skipping it would leave the hash's wire state non-uniform at the moment
  of removal, which is exactly what §9.3's removal gate forbids. The result is
  the state the index was already half
  claiming: every reference to `h` tombstoned, no blob, global availability
  truthful again. The mixed pair is admitted past this X11 refusal — after the
  strict X1–X10 wire decode — **only** for a confirmed selection that is total
  over `h`, that is `--blob <h> --yes` (repeated once per mixed hash where more
  than one exists) or an applicable `--all --yes`.
- **The direction is deliberate.** The alternative repair — un-tombstone the
  tombstoned reference so it agrees with the live one — is rejected: it would
  make tpatch adopt bytes into a reference the operator purged, silently
  reversing a confirmed destructive decision on the strength of an index edit
  tpatch did not make. Purging in the direction the tombstone already recorded
  destroys nothing the operator had not already asked to destroy for that hash,
  and it is the direction §9.7.1's global selection rule already implements.
- **If any reference to `h` is removal-pending, this classification does not
  apply at all: the hash is owned.** A purge transaction is in flight over `h`,
  ownership outranks every other classification of that hash, and pending
  recovery owns the index before any new selection (§9.3, §9.7). Every
  observer other than `purge --yes` reports `recovery-pending` and routes to the
  owner; `purge <slug> --blob <h> --yes` claims `h` globally, finalizes the
  pending references terminally, and the operator reruns the same selector. The
  compound ordering, and why it terminates, is stated in §9.7.2.

**Neither subcase is `archive-purge-evidence-divergent`.** That code requires
genuinely divergent evidence **under a pending reference** — a hash-wrong or
non-regular blob, or an index that stopped strict-decoding mid-run. Neither
subcase is routed to §9.7.2's pending+absent escape, and neither is ever routed
to `--abandon-transaction`, and the same is true of the retained corrupt-blob
observation above (PIB-521, PIB-522, PIB-523, PIB-524, PIB-531,
PIB-532, PIB-533, PIB-535, PIB-543).

This keeps strict shape decoding while making every documented repair reachable
rather than permanently bricking the archive
(PIB-331…PIB-341, PIB-401, PIB-428, PIB-444, PIB-457…PIB-460,
PIB-521…PIB-524, PIB-531…PIB-533, PIB-535, PIB-539…PIB-543).

### 9.4 Determinism

Two invocations that replace the same artifacts with the same prior bytes
produce **byte-identical** blobs and a byte-identical `index.json` entry. The
whole archive is a pure function of the sequence of replaced content — no
clock, no PID, no hostname, no random suffix reaches it (the random suffix is
confined to the gitignored staging tree). PIB-160, PIB-161, PIB-162.

### 9.5 Recovery UX in v1

The success report names, for each replaced artifact, the repo-relative blob
path. Recovery is an ordinary file copy:

```text
cp .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob \
   .tpatch/features/<slug>/spec.md
```

No `--restore` verb ships in v1 (non-goal 6, Q4). The report must state the
copy form explicitly and must not imply a verb that does not exist (PIB-163).

### 9.6 Privacy: the redaction precondition and the retention disclosure

#### 9.6.1 ADR-027 D3 applies, and it is a hard gate

ADR-027 D3 is unambiguous: *"Every tpatch-owned writer MUST run the redaction
contract before persisting any capture artifact in either lane"*, and
*"Redaction failure is a hard failure for committed summaries"*
(`docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-177`). The archive
is a tpatch-owned writer persisting bytes into the **tracked** lane. rev-0 did
not engage D3 at all.

**The gate** (ADR-035 D15): before any blob is written, each `replace` entry's prior bytes are
passed to the shipped scanner `redact.Scan`
(`internal/redact/redact.go:81-90`), which applies the closed six-class
resource policy (`internal/redact/redact.go:53-72`): `private-key`,
`connection-url`, `email-pii`, `credential-assignment`, `bearer-or-key-token`,
`home-absolute-path`.

**On a match, the whole invocation refuses**: exit 3,
`archive-content-refused-sensitive`, zero blobs written, zero canonical
mutations, and the report names **the artifact and the matched class codes —
never the matched bytes, never an excerpt, never an offset** (PIB-262 …
PIB-267).

Refusal rather than scrubbing is forced by the archive's purpose. Scrubbing
produces bytes that are not the file's bytes, so a "recovery" copy would
silently differ from what was replaced — a worse failure than refusing. The two
requirements (exact recovery, and never persisting secret-shaped content) are
genuinely in conflict, and the design resolves the conflict by refusing the
*destructive* operation, never by silently persisting the secret. `redact.Scan`
takes in-memory bytes and never a path precisely so this ordering is structural
rather than a call-site discipline (`internal/redact/redact.go:18-21`).

**The operator's route forward** is stated in the refusal itself: remove the
sensitive material from the artifact (or move the file aside yourself), then
re-run. The refusal never suggests `--force`, because none exists.

**A false positive is possible and is not silently overridable.**
`home-absolute-path` **and the broad empirically relevant `email-pii` class** can match legitimate Markdown examples or contact text inside a hand-authored spec. The chosen trade is: a false positive costs the operator an
edit and a re-run; a false negative costs a credential retained in every future
clone. Q9 records a future scoped-override design; it is deliberately not in
v1, because an override flag on a redaction gate is exactly the shape that
turns a hard gate into a suggestion.

#### 9.6.2 The retention disclosure — rev-0's "no new exposure class" is withdrawn

rev-0 argued that archiving introduced no new exposure class because the same
content was already committable at `analysis.md`. That is **wrong**, and the
correction is normative:

1. **Overwriting used to be a removal.** Before this PRD, replacing `spec.md`
   destroyed the prior bytes. An operator who pasted a credential into a spec
   could remove it by overwriting the file. With the archive, the same
   `--regenerate` **preserves** those bytes at a second tracked path. The
   exposure class is not "different content"; it is **durable retention of
   content the operator was destroying**.
2. **Committed retention is durable across clones.** Once committed, a blob is
   in the history of every clone. That is the point of the recovery guarantee
   and the cost of it, and §9.7 is the bound.
3. **Tracking is conditional, and no clone-durability is claimed before a
   commit.** The archive is tracked **iff the operator tracks `.tpatch/`**.
   Where `.tpatch/` is untracked (which the CLI already detects,
   `internal/cli/cobra.go:3405-3407`), the archive is an ordinary untracked
   directory: it survives crashes and re-runs only absent external deletion, and it does
   **not** survive a fresh clone **or `git clean -fd` / `git clean -xfd`**. Both
   forms can remove the untracked `.tpatch/features/**` archive, not just `.tpatch/local/**`. No sentence in this PRD, in any report, or in
   any doc may claim clone-durable recovery for uncommitted blobs; advisory
   `bundle-untracked-in-git` states both clone and Git-clean risk truthfully (PIB-139,
   PIB-342, PIB-343, PIB-426).
4. **`land` sweeps the archive into the operator's commit** like any other
   file under the feature directory (`internal/cli/land.go:723-725`), so an
   operator who lands a feature after a `--regenerate` commits the blobs
   without a separate action. That is disclosed in the human report and in
   `docs/feature-layout.md`, and PIB-152 asserts the sweep behavior (PIB-344
   asserts the disclosure exists).

### 9.7 Retention: listing, purging, tombstones and orphans

The retention surface is:

```text
tpatch feature intent-archive list  <slug> [--json] [--quiet] [--path <dir>]
tpatch feature intent-archive purge <slug> (--blob <hash>... | --generation <id>... | --orphans | --all) [--yes] [--json] [--quiet] [--path <dir>]
```

`list` is read-only: it does not acquire the directory lock, run G1–G4 or
create state. It strictly validates X1–X10, reports X11 storage state
(`present`, `pending-remove`, `pending-finalize`, `purged`, `orphan`,
`mixed-reference`, `corrupt` or
`dangling`), and exit-3 refuses a malformed live reference, a dangling live
reference, a **mixed tombstone/live-reference** hash or a **corrupt** blob under
a retained reference while still
rendering only safe identifiers and remediation. Its exits are pinned per
observation: a **tombstoned reference whose named blob is present while the hash
is globally unreferenced** is exit **0**, rendered as `orphan` in the
`orphans:` section and carrying the literal
`tpatch feature intent-archive purge <slug> --orphans --yes` repair; the **same
tombstone beside a blob whose hash is still live** — another reference to it
retained — is exit **3**, rendered as `mixed-reference` with the
tombstoning and the live generation ids, and carries the literal
`tpatch feature intent-archive purge <slug> --blob <hash> --yes` repair; a
**retained reference whose blob is present but non-regular or hash-wrong** is
exit **3**, rendered as `corrupt`, and carries §9.3.1's type-total
removal-then-confirmed-purge procedure; an **owned** hash — any
reference to it removal-pending — is exit **0**, rendered as `pending-remove` or
`pending-finalize` for **every** reference to it, and carries the
`purge … --yes` route of its owning transaction (§9.3). Neither
tombstone-beside-blob subcase
ever carries the archive-divergence procedure (§9.3.1, PIB-521, PIB-531,
PIB-535, PIB-541, PIB-543).

**When observations of different exits coexist, `list` renders all of them and
takes the highest exit.** An archive can hold a globally unreferenced residue
(exit 0, `orphan`) **and** a mixed tombstone/live-reference hash (exit 3,
`mixed-reference`) at once — they are facts about different hashes, and rev-9
pinned each in isolation without saying what happens together. The rule is
fixed rather than left to implementation order: **`list` renders every safe
observation it made, each with its own storage token and its own literal
repair, and exits 3 if any observation is an exit-3 one**. It never truncates
the report at the first exit-3 observation, never suppresses the exit-0
observations because a worse one exists, and never downgrades to exit 0 because
a benign observation came last. `doctor`'s D9 reports the identical set with the
identical repairs, warning-only and writing nothing (§12.5). A harness therefore
gets one exit code meaning "at least one observation here is untrustworthy" plus
a complete inventory of what was seen, which is the only combination that lets
an operator repair an archive in one pass instead of discovering the next
inconsistency after each fix (PIB-541).

**Multiple instances of one class are rendered as one class with many
instances.** Three unreferenced residues are three `orphan` rows carrying the
**same** literal `--orphans --yes` repair, because one invocation of it clears
all three; two mixed hashes are two `mixed-reference` rows carrying
`--blob <h₁> --yes` and `--blob <h₂> --yes`, which the operator may pass to a
single repeated-selector invocation (§5.1, §9.7.1). The report never implies
that each instance needs its own separate run, and never renders a repair that
would clear only part of the class it belongs to (§9.3.1, PIB-548, PIB-549).

`purge` is
preview-by-default,
requires one mutually-exclusive selector and `--yes` to act; **the preview form
acquires no directory lock on any selector, and only `--yes` does** (§7.4.1,
§7.12, PIB-530). **Neither
subcommand runs a Git process**: `list` reads, and `purge` writes only the
tracked archive and never the gitignored staging lane, so the local-lane gate
has no subject in either, in **both** the preview and the `--yes` form
(§7.13, PIB-514).

**`purge` never recovers a prepare journal.** A pending journal means an
interrupted *publication* transaction whose undo evidence binds canonical
artifacts, `status.json` and the archive index together (§7.5). Recovering it
is `prepare`'s job, under `prepare`'s J1–J10 binds; a retention command that
consumed it would be undoing a publication the operator did not ask about, from
a verb whose subject is stored bytes. So:

- Before any archive work, `purge` checks for the slug's journal **marker**.
  If one exists it refuses exit 3, `recovery-pending`, with the whole tree
  byte-identical. It does not strict-decode the journal, does not move it, does
  not consume it and does not undo a single entry (PIB-350, PIB-489, PIB-490).
- The remediation names the same two routes `--dry-run` names: run a mutating
  `prepare <slug>`, which recovers the **journal** and reports `recovered`
  (§7.11), or
  `tpatch prepare <slug> --abandon-transaction --yes`. Neither is performed for
  the operator.
- The acting (`--yes`) path acquires the workspace authority first, so a live
  sibling still yields `transaction-in-progress` (PIB-349), and performs the
  marker check under that authority. The preview path acquires nothing and
  performs the same marker check from observed bytes.
- **Only when no journal exists does the acting (`--yes`) path run the §9.7.2
  pending-hash recovery, and it does so on every selector.** `--blob`,
  `--generation`, `--all` **and `--orphans`** all take this pass first. There is
  no selector that skips it and no selector that recovers a journal. The pass is
  terminal in exactly the way a journal recovery is: it finalizes the pending
  hashes, reports exit 0 `recovered`, and does **not** go on to process the
  selector in the same invocation. The operator re-runs the reported retry —
  which carries **their** selector, not a substituted one — to perform the purge
  they asked for (§9.7.2, PIB-491, PIB-527, PIB-528). **This pass also precedes
  the whole-index X11 scan**, and is the only thing that does: it completes a
  transaction that already owns its hash, so an unrelated inconsistency
  elsewhere in the index does not block it, while the rerun that follows is an
  ordinary new-selector invocation and is fully governed by that scan (§9.3.1,
  §9.7.2, PIB-546).

  `--orphans` is in that set even though it rewrites no index, and the reason is
  not symmetry. The orphan set is *derived from* the index's live-reference set,
  and a removal-pending reference is a reference whose liveness is mid-decision:
  finalize it and its blob may become an orphan; leave it and the same blob is
  still referenced. Scanning for orphans against an index with pending
  references would therefore compute a set from state that is about to change,
  which is the exact defect that makes recovery terminal in the first place. So
  the pending hashes are finalized first, terminally, and the rescan runs
  against the index the operator can now see (PIB-527).
- **The preview does not recover, and says so — on every selector.** With no
  journal but one or more removal-pending hashes in the index, a `purge`
  without `--yes`, on **any** selector including `--orphans`, acquires
  no lock, writes nothing, removes nothing and finalizes nothing. It exits 0
  with the closed outcome token `recovery-required` (§10.2)
  and prints, in this order: that pending purge recovery is required before any
  new purge can run; each pending hash and the repo-relative
  `blobs/<hash>.blob` and `index.json` paths the recovery would act on; the
  per-hash plan the recovery would follow (claim every reference to that hash,
  remove the blob if present, then tombstone every reference to that hash); and
  the exact rerun that performs
  it — **the operator's own command with `--yes` appended**, run from the
  workspace root under the one verbatim heading of §9.7.2, carrying
  `retry_cwd: "workspace-root"` like every other retry this PRD emits. It never
  emits `outcome: "recovered"`, because it recovered nothing. The human form,
  for an operator who ran `purge <slug> --all`, is:

  ```text
  feature intent-archive purge <slug>: recovery required before this purge
    A previous purge stopped with pending references. Nothing was changed.
      pending hash: <hash>
        blob:  .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob
        index: .tpatch/features/<slug>/artifacts/intent-archive/index.json
        plan:  claim every reference to it, remove the blob if it is present, then tombstone every reference to it
  Run this again from the same workspace root:
    tpatch feature intent-archive purge <slug> --all --yes
  ```

  The heading is at **column 0** of the emitted output, with no leading or
  trailing whitespace and nothing else on its line, exactly as §9.7.2 requires
  of every emitter (PIB-498).

  **Selector preservation is normative, and it is the whole reason the retry is
  trustworthy.** The last line reproduces the selector the operator actually
  passed — `--all` here, `--orphans` for an orphan scan, `--blob <h>` with the
  operator's own hashes, `--generation <id>` with their own ids — with `--yes`
  appended and **no** root-selection argv, exactly as §9.7.2's sanitization rule
  requires of every retry. It is never widened to `--all` for convenience and
  never narrowed to the pending hash, because either substitution would name a
  command that does something other than what the operator asked, on the one
  line they are most likely to copy and run unread. The recovery that `--yes`
  then performs is identical on every selector — it is a property of the index,
  not of the selection — so preserving the selector costs nothing and keeps the
  reported command true (PIB-528, PIB-529).

  Only the `--yes` form takes the authority and performs the terminal recovery.
  PIB-515 asserts the exit code, the `recovery-required` outcome token, the zero
  writes, the **zero** flock
  acquisitions, the listed hash/plan and the absence of any `recovered`
  outcome; PIB-529 asserts the JSON object and its parity with the human form.

#### 9.7.1 Selection and shared references

**Selection narrows what is acted on; it never narrows what is validated.**
X11's storage observation runs over the **whole index** before every archive
mutation a new selector would perform, independent of the selector (§9.3.1), so
everything below describes
only which references a selector *acts on* once the archive as a whole has been
found consistent — or found to hold exactly one repair class that the confirmed
selection covers completely. A global inconsistency outside that class refuses
every selector,
zero-write, naming that observation's owning repair (PIB-542, PIB-549).
Completing an already-pending purge transaction runs before that scan and is its
one exception (§9.3.1, §9.7.2, PIB-546).

Reference counting is global over every reference with equal `content_sha256`,
retained and removal-pending alike — those are exactly the references that make
a hash **live** (§9.3); a pending hash makes its hash **owned** and is recovered
before new selection. **An already-tombstoned reference is not a no-op for
selection**: when the selector is total over its hash it is claimed into
removal-pending with every other reference to that hash and tombstoned again at
the end, because §9.3's removal gate requires every reference to the hash to be
removal-pending at the moment of removal and a skipped reference would block it
forever (§9.3, §9.7.2, PIB-532, PIB-545).

- `--blob h` selects **every** reference to `h`. It first marks every selected
  reference — retained and already-tombstoned alike — pending in one CAS
  rewrite, the same **global claim**
  §9.7.2's recovery makes over a hash it adopts, then completes the one-hash
  state machine in §9.7.2. The flag **repeats** (§5.1), so
  `--blob h₁ --blob h₂ --yes` performs the same total-over-the-hash treatment
  for each named hash in lexical order and is how one invocation covers every
  instance of a same-class multi-hash repair (PIB-548). This is the exact
  two-command accidental-secret
  route.
- `--generation id` selects that generation's references. If any selected hash
  is also referenced by a non-selected generation, it refuses
  `archive-blob-shared` (exit 3), names the hash and exact escalation
  `--blob <hash>` or `--all`; it never makes a tombstone lie or silently broadens
  selection.
- `--all` selects every retained reference and is allowed to remove every blob.
  Its lexically ordered per-hash processing remains valid after the first
  removal because every intermediate record is either retained, pending or
  tombstoned by the same state machine.
- `--orphans` selects only physical blobs whose hash is **globally
  unreferenced** — no reference to it, in any generation, is retained or
  removal-pending (§9.3) — and rewrites no index. It selects **every** such blob
  in the archive, not one: an archive holding three unreferenced residues is
  repaired by one `--orphans --yes`, and the multiplicity is what makes the
  admission usable (PIB-548). A blob whose hash is still
  live is **never** in that set, whatever any single reference to it says: a
  hash that one generation tombstones while another retains it is live, its
  blob is that live reference's storage, and `--orphans --yes` must leave it
  exactly where it is, because removing it would convert a retained reference
  into a dangling one and destroy recoverable bytes through the one selector
  that promises to remove nothing anybody references (§9.3.1 subcase B,
  PIB-531, PIB-533). `--orphans` is nonetheless preceded by the same terminal
  pending-hash recovery as every other selector (§9.7), because the orphan set
  is derived from a reference set that a pending reference is in the middle of
  changing; and it is the **one shipped repair** for a tombstoned reference
  whose named blob is present **and whose hash is unreferenced**, which it is
  explicitly admitted past X11 to remove (§9.3.1, §9.7.3, PIB-522, PIB-527).
  That admission is scoped to **exactly that repair class, covered
  completely**: `--orphans --yes`
  is not admitted past a mixed hash, a dangling retained reference, a corrupt
  blob or an unsafe pending blob, so an archive holding any of those
  refuses `--orphans --yes` at exit 3, zero-write, naming the other
  observation's repair — it does not remove the orphans it could have removed
  and report the rest as leftovers (§9.3.1, PIB-542, PIB-549).

A purged blob is not recoverable **until identical content is archived again**.
If identical content is later rehydrated, every tombstoned or removal-pending
reference to its hash is un-tombstoned in the same index rewrite. A missing
retained blob has exactly **one** repair: the confirmed global-hash purge named
in §9.3.1 and §9.7.3, which tombstones every reference to that hash without
attempting a removal. A tombstone beside a blob whose hash is **live but not
owned** likewise has exactly one repair, and it is the same confirmed
global-hash purge (§9.3.1 subcase B); a present but unidentifiable blob has
exactly one route too, and it is §9.3.1's type-total
removal-then-confirmed-purge or restore procedure. List/doctor/report
therefore never report a hash as
unavailable while a retained or removal-pending reference to it exists, never
say that bytes are absent while a live blob exists, and never offer a repair an
operator cannot reach
(PIB-423…PIB-425, PIB-444, PIB-457…PIB-460, PIB-531, PIB-543).

#### 9.7.2 Honest purge procedure and residual race

For **every** selector — `--blob`, `--generation`, `--all` and `--orphans` —
and **only** in the `--yes` form, and only after the journal-marker
refusal of §9.7 has passed, the authority runs `RecoverPendingPurge` **under the
workspace lock and before any new archive mutation**. That call site is the
**only** one in the shipped binary: no mutating `prepare` mode, no preview and
no diagnostic reaches it (§7.8 step 5, §7.11, PIB-526). It strict-decodes X1–X10
and applies X11's pending-state rules.

**It runs before the global X11 scan, and that ordering is the one explicit
exception to §9.3.1's pre-mutation validation rule.** §9.3.1 requires the
whole-index storage observation before every mutation a **new selector** would
perform. This pass is not that: it completes a transaction whose first mutation
already landed and whose hash is already owned. Concretely, the `--yes` path is:

1. journal-marker refusal (§9.7) — unchanged, and it precedes everything;
2. **`RecoverPendingPurge` over the owned hashes**, which strict-decodes
   X1–X10 and applies X11's rules **to the hashes it owns**. It may mutate and
   finalize those hashes while an unrelated inconsistency `h₂` sits elsewhere in
   the index, and it is terminal: exit 0 `recovered`, no selector processing;
3. only on a **rerun**, with nothing pending, the complete preflight below —
   including the whole-index X11 scan — which may then refuse on `h₂` at exit 3
   zero-write, naming `h₂`'s own repair.

The exception is narrow in three ways that PIB-546 asserts directly: it applies
only to hashes this transaction already owns, it never proceeds into the new
selector, and it belongs to `purge --yes` alone — a normal mutating `prepare`
observing the same index refuses exit 3 `recovery-pending` and finalizes nothing
(§7.8 step 5, PIB-525, PIB-526). Ordering it the other way round was rejected: a
global X11 refusal ahead of the recovery would let an unrelated residue
permanently block the completion of a purge whose bytes are already half
removed, which is a validation gate bricking the transaction it was added to
protect (§21).

**Recovery acts on the hash, not on the reference that carries the flag.** A
removal-pending reference means the purge transaction owns `h` globally
(§9.3), so the machine below is defined over *every* reference to `h` in every
generation and its central invariant is stated before its steps rather than
derived from them:

> **`blobs/h.blob` is removed only when the index, as re-read and revalidated
> immediately before the removal, holds no reference to `h` that is not
> removal-pending.** No removal ever happens while a retained reference to `h`
> survives, and no same-hash reference — retained or already tombstoned — is
> exempt from the claim that establishes it.

For every pending hash, sorted lexically, it does exactly this:

1. **Claim `h` globally.** Re-read the index under the authority. If `h.blob` is
   present, regular and hash-correct **and** any reference to `h` is not yet
   removal-pending, CAS-publish **one** index rewrite that sets every reference
   to `h`, in every generation and in every wire state, to removal-pending —
   retained references
   because they are being swept into the transaction that owns their hash, and
   already-tombstoned references because their end state is unchanged and the
   claim is what makes the hash's wire state uniform before any byte is
   removed — and fsync the index directory. **No same-hash reference is exempt
   and none is treated as a no-op**; the only condition under which this step
   writes nothing is that *every* reference to `h` is **already**
   removal-pending, which is the same postcondition reached by a different
   route (§9.3, PIB-532, PIB-545).
2. **Revalidate immediately before acting.** Re-read the index and re-observe
   `blobs/h.blob` through the held root. Removal proceeds only when **every**
   reference to `h` is removal-pending **and** the file is a regular file whose
   bytes hash to `h`. Any other observation does not remove.
3. **Remove.** Rooted removal of `h.blob`, then fsync `blobs/`.
4. **Tombstone.** CAS-publish **every** reference to `h`, in every generation,
   as tombstoned in one index rewrite, and fsync the index directory.
5. **The absent-blob path.** If `h.blob` is absent at step 1, the removal has
   already happened and no claim against present bytes is possible or needed: a
   single CAS-published rewrite sets **every** reference to `h` — pending,
   already-tombstoned, and any retained reference that appeared after the
   removal — **directly to tombstoned**, and **no** removal is attempted and no
   intermediate pending rewrite is published, because there are no bytes left
   for a claim to protect. That is
   byte-for-byte the shipped dangling repair of §9.3.1, which is why this path
   needs no separate route: the index ends up saying exactly what is true, that
   these bytes are gone. **No retained reference survives it** — the rewrite is
   total over the hash, so the path cannot leave behind the dangling reference
   it exists to clear (PIB-540, PIB-545).
6. **Unsafe or wrong evidence.** If `h.blob` is present but non-regular or
   hash-wrong under the claim, refuse without rewriting evidence
   (`archive-purge-evidence-divergent`, §10.4.1).

Steps 1–2 are what rev-10 adds, and they are not defensive decoration. rev-9
acted on the pending references only, so an index in which `G1` referenced `h`
as retained while `G2`'s reference to `h` was removal-pending would have had
`h.blob` removed with `G1` left retained — a dangling reference manufactured by
the recovery itself. rev-9's own compound ladder acknowledged that outcome and
routed it to the dangling repair. Rev-10 removes the composition instead: the
claim in step 1 makes `G1` part of the transaction *before* the bytes go, and
step 2 refuses to remove if anything about that has changed (PIB-539, PIB-540).
**Rev-11 closes the last gap in that argument**: rev-10 exempted
already-tombstoned references from the claim in two neighbouring sections while
step 2 gated the removal on *every* reference being pending, so an index holding
one tombstoned and one retained reference to `h` could never satisfy the gate
and the hash would have been stuck pending forever. The exemption is withdrawn,
not the gate (PIB-532, PIB-545).

**External same-hash reference insertion and external replacement, modelled per
window.** The workspace
authority excludes tpatch mutators only (§7.12), so an external writer — an
editor, a hand-merge, a `git checkout` of an older `index.json`, a script that
recreates `blobs/h.blob` — can change the index or the file while this machine
runs. The five windows are enumerated,
and the residual is disclosed rather than claimed away:

| When the external write lands | What happens |
|---|---|
| A reference to `h` is inserted before step 1's re-read | It is an ordinary part of the observed index: step 1's claim includes it, and it is normalized to removal-pending with every other reference to `h`. Nothing special occurs |
| A reference to `h` is inserted between step 1's re-read and its CAS | The CAS compares the captured index preimage and **fails**. Nothing is removed. Because a pending record already exists, this machine is by definition inside the post-first-mutation window (see below), so the invocation ends in `archive-purge-partial` (exit 5) on branch (a) with `pending_hash: h`; the sanitized retry re-reads the new index and claims `h` again, now including the inserted reference. The same insertion against a *new* selection that has not yet written is instead the preflight's exit-3 `archive-purge-index-changed`, zero-write (§10.4.1) |
| A reference to `h` is inserted between step 1's CAS and step 2's revalidation | Step 2 observes a reference to `h` that is not removal-pending and **does not remove**. This is an externally-caused revalidation failure after the first mutation, so it is `archive-purge-partial` (exit 5) on branch (a) with `pending_hash: h`; the reported retry re-enters the machine, and its step 1 claims the inserted reference. The loop is the operator's retry, not an unbounded in-process retry, so the command always terminates and always reports what it saw |
| **The file at `blobs/h.blob` is replaced between step 2's revalidation and step 3's unlink** | **Not detected, and not claimed to be.** The revalidation reads the file and the unlink names the path; there is no primitive on either supported platform that removes a path only if its contents still hash to a value observed a moment earlier, and this PRD does not pretend otherwise. Step 3 removes whatever object is at that path — the replacement, not the validated bytes. This is the same disclosed **final-syscall residual** the canonical publication carries (§7.1, §7.8), narrowed to one syscall gap and stated here so it is not discovered later as a silent loss. Its consequence is bounded: the hash is being purged, so the index's end state (every reference tombstoned, no blob) is truthful either way; what the operator loses is bytes an external writer put there after tpatch had already decided to remove that path. No shipped message claims this window is closed, and PIB-550 fails any that does |
| A reference to `h` is inserted after step 3's removal — i.e. against a hash whose bytes are already gone | The step 4 tombstone CAS detects it and yields exit 5 as above; the retry then takes step 5's absent-blob path and tombstones every reference to `h`, the inserted one included, which is the truthful outcome because the bytes really are gone. A write that lands inside the CAS→rename final syscall window is **not** detected — the same disclosed post-CAS residual as canonical publication (§9.7.2's closing paragraph) — and leaves a retained reference to an absent blob, which X11 classifies `archive-blob-dangling` with the confirmed `--blob <h> --yes` repair it already owns |

No window produces a silent index inconsistency and none produces a dead end:
every one either
completes, or reports a resumable outcome whose retry re-derives the claim, or
lands in an existing classification with an existing repair. **Two of them do
produce a disclosed byte-level residual** — the revalidate→unlink replacement
above and the post-CAS rename window below — and both are stated as residuals
rather than resolved, because the lock excludes tpatch mutators only
(PIB-540, PIB-544, PIB-550).

This recovery is idempotent: a crash before the claim repeats step 1; a crash
after the claim but before removal repeats steps 2–4; a crash
after removal but before the tombstone takes step 5; a crash after the tombstone
advances to the next hash. It has no restore branch and no permanent
X11 brick.

**If it did anything, the invocation ends there.** When at least one pending
hash was finalized, the command returns exit 0 with `outcome: "recovered"`, a
`recovery` object of kind `archive-purge-finalize` naming the finalized hashes,
and the sanitized equivalent retry for the selector the operator passed —
**their** selector, `--orphans` included, never a substituted one (§9.7). It does
**not** continue into a new purge in the same invocation: the selection was
computed against an index that the recovery has just rewritten, and re-deriving
it silently would make the reported result depend on state the operator never
saw. A mutating `prepare` in the same state does not reach this pass at all: it
refuses exit 3 `recovery-pending` and names this command (§7.8 step 5, §7.11).
When nothing was pending, this pass is a no-op and
evaluation continues into the preflight below (PIB-486, PIB-491, PIB-527,
PIB-528).

Only after that recovery does purge run its **complete preflight**, before the
first byte of new mutation. Everything predictable is decided here, so the
zero-write refusal population is as large as it can honestly be. **The
recovery's precedence over this preflight is deliberate and is the sole
exception to §9.3.1's pre-mutation validation rule**: a transaction that already
owns a hash finishes first, terminally; the whole-index scan below governs
everything a *new* selector would do (§9.3.1, PIB-546):

| Preflight check | Refusal on failure |
|---|---|
| selector present, exactly one, and well-formed (`^[0-9a-f]{64}$` hashes, known generation ids) | exit 1 (no selector) / exit 3 (`archive-index-corrupt` for an unknown id) |
| strict index decode X1–X10 over the captured raw preimage | exit 3, the matching `archive-index-*` code |
| X11 storage observation over the **whole index**, never over the selection — every reference in every generation, retained/pending/tombstoned versus present/absent/regular/hash-correct, with each hash's **global** ownership and liveness computed once (§9.3, §9.3.1) | exit 3, `archive-blob-dangling`, `archive-blob-corrupt` or `archive-index-storage-inconsistent` — with exactly **three** admitted exceptions, each of which is one observed **repair class**'s one shipped repair, each admitted **only** for that class and only when the confirmed selection **covers every instance of that class in the whole index**, and only when it is the *sole* class the global scan found: dangling retained hashes, where the confirmed selection is exactly those hashes (`--blob <h> --yes`, repeated per hash, or an applicable `--all --yes`), are admitted (§9.7.3); **tombstoned references whose named blobs are present while their hashes are globally unreferenced** are admitted under `--orphans --yes`, which removes every such unreferenced blob and rewrites no index (§9.3.1 subcase A); and the **same tombstones beside blobs whose hashes are still live** are admitted only under a confirmed selection total over every one of those hashes (`--blob <h> --yes` per hash, or an applicable `--all --yes`), which purges each hash globally and never under `--orphans` (§9.3.1 subcase B). All three admissions happen **after** the strict X1–X10 wire decode above, never before it. An inconsistency of a **different class** anywhere in the index, or an instance of the same class the selection does not cover, withdraws every admission: the invocation refuses exit 3, zero-write, and names each class's own repair (PIB-542, PIB-548, PIB-549) |
| shared-reference analysis: a `--generation` selection whose hash is referenced by a non-selected generation | exit 3, `archive-blob-shared`, naming `--blob <hash>` or `--all` |
| per-orphan identity and hash for `--orphans`, over the globally unreferenced set only | exit 3, `archive-blob-corrupt` |
| global live-reference count per selected hash, computed once from the validated index | exit 3, the matching code |

Every one of those exits **before the first write**, and PIB-465 asserts each
with a whole-tree byte snapshot.

The command then makes the selected unique hashes a lexically sorted work list.
Selection is always **total over each selected hash** — `--blob h` and `--all`
select every reference to `h` by definition, and `--generation` refuses
`archive-blob-shared` rather than selecting part of one — so the CAS below is
the same global claim the recovery makes. For each hash, it CAS-publishes
**every** reference to that hash — retained and already-tombstoned alike — as
removal-pending and fsyncs; it then
immediately runs the recovery steps above from step 2. A generation/`--all`
purge therefore
never produces an invalid state after the first hash is removed, and never
removes a blob while a retained reference to it survives. For a dangling
retained `h`, a confirmed global `--blob h --yes` (or applicable `--all --yes`)
CAS-publishes its references straight to tombstones only after rechecking that
the blob remains absent; it never invents a removal. For a **mixed
tombstone/live-reference** `h` (§9.3.1 subcase B), the same confirmed selection
runs the ordinary machine: the claim sweeps the retained **and** the
already-tombstoned references into removal-pending in one rewrite, the blob is
revalidated and removed, and every reference to `h` is then tombstoned — after
which the hash's global availability matches
what the index says. For a **corrupt** `h` (§9.3.1) no selector is
admitted at all: the operator's own type-total `rm -rf --` of the managed blob
path removes the
unidentifiable object first, after which the confirmed selection is the ordinary
dangling repair. `--orphans` similarly
captures and revalidates its strict index preimage and each orphan
identity/hash immediately before removal, over the globally unreferenced set
only, and it processes **every** member of that set in one invocation.

**Compound state: a pending hash *and* a residue observation in the same
archive.** These are independent facts about different hashes, and the ordering
between them is fixed rather than left to the selector. Take an index holding a
removal-pending reference to `h₁` **and** a tombstone-beside-present-blob
observation on `h₂` (either subcase). Then, for any selector:

1. **Preview** (`purge <slug> <selector>`, no `--yes`) acquires no lock, writes
   nothing and reports `recovery-required` for `h₁` with the retry that is the
   operator's own selector plus `--yes` (§9.7). It does **not** report the
   `h₂` observation as the thing to fix first, because pending recovery runs
   first and can change what the rescan sees.
2. **The `--yes` run** performs the terminal pending recovery for `h₁` only:
   exit 0, `outcome: "recovered"`, `recovery.kind: "archive-purge-finalize"`,
   and the reported retry is again the operator's own selector. `h₂` is
   untouched — the recovery finalizes pending hashes and nothing else.
   **`h₂`'s inconsistency does not block this step**, and that is the ordering
   exception stated above: the whole-index X11 scan governs new-selector
   mutation, and this run performs none. It claims, removes and tombstones only
   `h₁`, whose hash the transaction already owned before the invocation began
   (§9.3.1, §9.7.2's ordering block, PIB-546).
3. **The rerun** then reaches the preflight with no pending hash left, performs
   the full global X11 scan, observes
   `h₂`, and behaves exactly as §9.3.1 defines for the subcase: under
   `--orphans --yes` with `h₂` unreferenced it is the admitted repair and
   removes the residue at exit 0 — together with every other unreferenced
   residue in the archive, since the admission is per class and the selector
   covers the class; under a confirmed selection total over `h₂`
   with `h₂` live it is the admitted repair and purges the hash globally at
   exit 0; under any **other** selector it refuses exit 3
   `archive-index-storage-inconsistent`, writing nothing, and names that
   subcase's literal repair command. The admission is for `h₂`'s repair
   **class**: an instance of the same class the selector does not cover, or an
   inconsistency of a *different* class, makes the rerun refuse
   zero-write and name each class's own repair (§9.3.1, PIB-542, PIB-549).

There is no dead end in that ladder, and in particular the refusal at step 3 is
not one: it names an executable command, and running it completes the archive's
repair. rev-9 recorded one composition here that *did* look like a dead end —
pending recovery removing `h₁`'s blob while another reference to `h₁` remained
retained, leaving a dangling reference to repair afterwards. **That composition
no longer exists.** The recovery claims `h₁` globally before it removes anything
and revalidates immediately before the removal, so a retained reference to `h₁`
is either swept into the transaction and tombstoned with the rest, or the
removal does not happen at all (§9.3, the invariant above). The only way a
retained reference to `h₁` can survive the removal is an external write inside
the disclosed post-CAS final syscall window, which is the residual every
tpatch path carries and which lands in `archive-blob-dangling` with the
confirmed `purge <slug> --blob h₁ --yes` repair §9.3.1 already names.
**And rev-10's own version of this ladder had a second dead end, which rev-11
removes**: it said `h₂`'s presence did not block step 2 while §9.3.1 said the
global scan preceded *every* archive mutation, which — read literally — made the
`--yes` run refuse on `h₂` and left `h₁` pending forever, with the only
"repair" being the very command that had just refused. The ordering exception
above is what makes step 2 executable, and PIB-546 asserts it against exactly
that fixture
(PIB-534, PIB-539, PIB-540, PIB-546).

**After the first mutation, failure is a distinct retryable outcome — not exit
3.** Once the first removal-pending index rewrite, tombstone rewrite or blob
removal has landed, the command has written, so the exit-3 promise of "wrote
nothing" no longer holds for it. A later I/O error, fsync failure or
externally-caused revalidation failure therefore ends in
**`archive-purge-partial`**, and the report states exactly where the per-hash
machine stopped.

**"After the first mutation" is a property of the purge transaction, not of the
current process.** The mutation that opens this window is the first one of the
per-hash machine that is **in flight** — which is very often a write performed
by an *earlier* invocation whose pending record this run is resuming. A
`purge --yes` that acquires the authority, finds a removal-pending reference and
then hits an I/O error inside `RecoverPendingPurge` before writing a byte of its
own is still in the post-first-mutation window: the transaction it adopted had
already written, the index has already advanced, and reporting exit 3 for it
would claim a zero-write that the transaction as a whole cannot honour. So both
`archive-purge-partial` (exit 5) and `archive-purge-evidence-divergent`
(exit 6) are reachable from a recovery invocation that performed no write
itself, and neither is reachable before the pending record exists. This is also
why both codes are **purge-owned**: `purge --yes` is the only command that can
open, adopt, advance or discover that transaction at all (§7.8 step 5, §7.11,
§10.4.1, PIB-526).

The partial report's fields are:

| Field | Meaning |
|---|---|
| `completed_hashes[]` | hashes whose references are fully tombstoned and whose blob is gone; nothing remains to do for them. For `--orphans`, the orphan blobs already removed |
| `pending_hash` | present **only** on branch (a) below: the single hash whose references are removal-pending, i.e. the one a recovery run resumes. Absent on branches (b) and (c), and the field's presence is exactly the branch discriminator |
| `remaining_hashes[]` | selected hashes not yet started, still fully retained. For `--orphans`, the orphan blobs not yet removed |
| `resume` | the closed field naming which of the three branches below applies: `pending-recovery-then-completion`, `completion-only`, or `orphan-scan`. It is what makes the retry prose a statement about *this* run rather than a general story (§10.2) |
| `retry` | the **equivalent command**, without any root-selection argv, because the per-hash machine is resumable and idempotent through the recovery steps above. What that retry *does* depends on `resume` and is stated per branch below (see "Retries are sanitized") |
| `retry_cwd` | the closed literal `"workspace-root"`: where that retry must be run from |
| `state` | the one-line statement that the index currently decodes, is internally consistent, and claims no absent bytes |

**How many runs finish it depends on where it stopped, and the report says
which case it is.** rev-6 told one story — "one `recovered` run, then one
completing run" — for three genuinely different stopping points. Two of them
have no pending marker at all, so promising a `recovered` outcome for them was
false. The three branches are closed and total over the partial outcome:

**(a) `resume: "pending-recovery-then-completion"` — a pending hash exists.**
The failure landed inside one hash's machine, after its removal-pending CAS.
`pending_hash` is that hash. The first retry finds it pending, runs the terminal
per-hash recovery above, and ends at exit 0 `outcome: "recovered"` with the
finalized hash named — it does **not** process the selector. A second run of the
same command then processes the remaining work list and completes the purge.
The report states both runs explicitly, so a second `recovered` is never a
surprise (PIB-466, PIB-467).

**(b) `resume: "completion-only"` — the failure landed exactly between two
hashes.** One hash's machine had finished (blob gone, every reference
tombstoned) and the next had not started, so **no** reference is
removal-pending and `pending_hash` is absent. There is nothing to recover: a
single retry re-derives the selection, observes that the already-completed
hashes are **already in their end state** — every reference tombstoned and the
blob gone, which is what the machine would have produced — and processes
`remaining_hashes` to
completion at exit 0. That is a *completed-hash* observation, not the
"tombstoned reference is a no-op" rule rev-10 wrongly generalized from it: a
tombstoned reference to a hash the current selection is still working on is
claimed like every other (§9.7.1, §9.7.2 step 1). The report promises exactly
one run and
**must not** promise a `recovered` outcome, because no recovery will occur
(PIB-516).

**(c) `resume: "orphan-scan"` — an `--orphans` purge stopped after its first
removal.** `--orphans` rewrites no index (§9.7.1), so it has no pending state to
carry and `pending_hash` is absent; `index.json` is byte-identical to its
pre-run bytes. A single retry recaptures and revalidates the index preimage,
re-derives the orphan set from the current filesystem — which no longer contains
the blobs the first run removed — and removes what remains, at exit 0. Again one
run, and again no `recovered` outcome is promised (PIB-517).

PIB-518 is the guard that binds the shape to the branch: `pending_hash` is
present if and only if `resume` is `pending-recovery-then-completion`, and a
report whose prose promises a `recovered` run on a `completion-only` or
`orphan-scan` branch fails it.

**Retries are sanitized: no report ever echoes an inherited `--path`.** The
operator may have selected the workspace with the inherited persistent
`--path <dir>` flag (`internal/cli/cobra.go:66`), whose value is an **absolute**
pathname in ordinary use. rev-5 promised "the exact same command line the
operator ran, verbatim", which would render that absolute path into a report
that §10.6 and §10.7 forbid absolute paths in, and would do so on the one code
path an operator is most likely to copy, paste and paste again into a bug
report. The rule is therefore uniform across **every** retry this cluster
emits — the partial-purge retry here, both terminal-recovery retries of §7.11,
the pending-purge preview retry of §9.7, the archive-divergence rerun below,
the dangling-reference repair of §9.7.3, both tombstone-beside-blob repairs
of §9.3.1, and §9.3.1's corrupt-blob repair:

1. `retry` is the **equivalent** command: same verb, same subcommand, same
   slug, same selector, same mode and confirmation flags — and **no**
   root-selection argv. `--path` is never reproduced, never rewritten to a
   relative form, and never replaced by a placeholder that looks like a path.
2. `retry_cwd` is the closed field whose only v1 value is the literal
   `"workspace-root"`. It says what the omitted argv used to say.
3. The human rendering states it in one sentence, verbatim, and that sentence
   is emitted at **column 0** — no leading whitespace, no trailing whitespace,
   nothing else on the line — with the `retry` line immediately beneath it and
   nothing between the two. The literal, between the delimiters, is exactly:

```text
Run this again from the same workspace root:
```

**The heading's position is part of the literal, not presentation.** Every other
line of these reports is indented to show structure, and rev-9's own worked
examples indented the heading with them — which made the "byte-for-byte"
promise untestable, because two emitters could agree on the sentence and
disagree on the two spaces in front of it, and a harness matching on the line
would then need to know each caller's nesting depth. Rev-10 fixes the position
with the text: the heading is written at column 0 by every emitter regardless of
how deeply the surrounding message is indented, it carries no trailing
whitespace, and no other token shares its line. The command line beneath it
keeps its own indentation, which is presentation and is not asserted. §9.7's
pending-purge preview, §9.7.2's divergence procedure and §10.7's recovery
example are all written that way, and PIB-498's sensitivity set now includes an
emitter that prints the correct sentence indented by two spaces and one that
prints it with a trailing space — both of which must fail (PIB-498).

**That sentence is the single heading, and there is no second spelling.** Every
human rendering in this cluster that hands the operator a `tpatch` command to
run — pending-purge preview, terminal journal recovery, terminal pending-purge
recovery, partial-purge resumption, the archive-divergence rerun, the
dangling-reference repair, §9.3.1's corrupt-blob repair, and both §9.3.1
residue repairs — prints exactly
that line, character for character, immediately above the command and nothing
else between them. rev-8 shipped a second spelling
(`Run this again from the same workspace root to perform it:`) on the preview
path and a numbered "3. run this again …" step inside the divergence procedure;
both are withdrawn, because a heading an operator's eye and a harness's matcher
both key on cannot have variants. No alternate punctuation, no appended
purpose clause, no numbered prefix, no capitalization variant, **no leading
indentation and no trailing whitespace** is permitted anywhere. The three external shell forms §10.7 permits (`cp` restore, lane
`rm -rf`, and the exact-path `rm -rf --` shared by the divergence procedure and
§9.3.1's corrupt-object repair) are **not** retries —
they are operator-executed procedures for things tpatch refuses to do — so they
keep their own labels and are outside this rule. PIB-498 is the total guard:
it derives the emitter set rather than checking a list, and its sensitivity
fixtures are rev-8's `to perform it:` variant, the withdrawn numbered step, a
correct sentence indented by two spaces and a correct sentence with a trailing
space (PIB-497, PIB-498, PIB-529).

The property is asserted directly rather than argued: PIB-497 runs each of
these paths with an absolute `--path` and asserts that the absolute string
appears nowhere in stdout, stderr or the JSON report, and PIB-498 derives every
emitter and asserts the `retry`/`retry_cwd` pair and the one verbatim sentence,
at column 0 and alone on its line, over all of them.

**Why the new code binds to exit 5.** Exit 3 must keep its zero-write meaning, so
it cannot carry this. Exit 6 means *manual intervention required* and is
reserved for evidence a program must not resolve. A partial purge is neither: it
is consistent, self-describing and resumable by re-running the equivalent
sanitized command, which is exactly exit 5's "transaction aborted, retry from
the observed tree" character. `archive-purge-partial` is therefore **exit 5**, and §10.4's
exit-5 row is amended to name its two populations explicitly — publication
abort with zero canonical/index change, and archive-purge partial progress —
rather than silently widening the older promise (PIB-466, PIB-467).

If the evidence itself is divergent or requires human judgement — a blob that is
present but hash-wrong under a **pending** reference, a non-regular file where a
pending blob belongs, or an index that no longer strict-decodes after an external
write — the outcome is **exit 6, `archive-purge-evidence-divergent`**, not 5,
and nothing further is attempted (PIB-468).

**This population is exactly the pending-hash and pending-index evidence, and
nothing else.** rev-7 also listed "a tombstone beside a live blob" here and
offered it this population's escape. That was unexecutable: a tombstoned
reference is not pending, so there is no pending hash to name, no purge
transaction in flight, and nothing for the escape's rerun to finalize — the
"pending+absent case" it points at cannot fire on a reference that is already
tombstoned. **Both** of §9.3.1's tombstone-beside-blob subcases are therefore
outside this population, and each has its own exit-3 zero-write classification
and its own literal repair: an **unreferenced** hash is physical residue,
repaired by `tpatch feature intent-archive purge <slug> --orphans --yes`, and a
still-**live** hash is a mixed tombstone/live-reference inconsistency, repaired
by the confirmed `tpatch feature intent-archive purge <slug> --blob <hash>
--yes` (§9.3.1, §9.7.3, PIB-521, PIB-522, PIB-531, PIB-532). A **retained**
reference beside a present but non-regular or hash-wrong blob is outside this
population for the same reason — no pending reference — and is exit-3
`archive-blob-corrupt` with §9.3.1's type-total removal-then-confirmed-purge or
restore route (PIB-543). None of the three is in this
population, and PIB-524 fails any classification that puts either
tombstone-beside-blob subcase back
(PIB-506, PIB-508).

**That exit 6 gets an archive-shaped escape, not the abandon mode.** rev-6
routed every exit-6 population to `tpatch prepare <slug> --abandon-transaction`.
For this code that is a dead end by construction: abandon touches no archive
index and no blob (§6.6 rules 8 and 10), so it cannot consume, move or resolve a
single byte of this evidence. Directing an operator there would hand them a
command guaranteed to refuse `no-pending-transaction`. This population therefore
gets its own procedure, and the message **never names abandon** (PIB-506,
PIB-508).

The command still refuses to act, because it will not remove or overwrite bytes
it cannot identify. What it does instead is report enough for the operator to
act, in repo-relative terms only. The procedure has two forms, selected by which
object diverged, and each is executable:

**Blob divergence** — under a **pending** reference, the blob is present but
hash-wrong, or is a non-regular file. The managed blob path is the divergent
object:

```text
feature intent-archive purge <slug>: refused archive-purge-evidence-divergent
  The object at this managed blob path is not the content the index records
  for it. Nothing was removed and nothing was rewritten.
    pending hash: <hash>
    blob:  .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob
    index: .tpatch/features/<slug>/artifacts/intent-archive/index.json
  tpatch will not delete or overwrite bytes it cannot identify. To finish this
  purge yourself, from the workspace root:
    1. this permanently deletes whatever object is at the managed blob path,
       including a directory and everything under it. There is no undo:
         rm -rf -- .tpatch/features/<slug>/artifacts/intent-archive/blobs/<hash>.blob
  If you want to keep that object, stop here and preserve it with a tool that
  matches what it is: cp -R for a directory, cp -P for a symlink, git show for
  a version-controlled original. tpatch does not print one copy command,
  because no single copy command is correct for every kind of object that can
  be at that path.
  The rerun below then finds the reference pending and the blob absent, which
  is the already-defined pending+absent case: it finalizes the tombstone and
  exits 0 recovered.
  What this costs: what you removed is gone, and this hash has no
  archived recovery material afterwards. If that blob was ever committed, it
  is still in this repository's Git history; removing it from history is not
  something tpatch does.
Run this again from the same workspace root:
  tpatch feature intent-archive purge <slug> --blob <hash> --yes
```

**Why one destructive command and no copy command.** This population's fixtures
include a hash-wrong regular file, a symlink, a directory, a FIFO and a device
node at the managed path (PIB-506). rev-10's `cp` + plain `rm` pair is
unexecutable against three of those and *misleading* against a fourth: `cp`
without `-R` and `rm` without `-r` both fail on a directory, and `cp` on a
symlink copies the target's bytes rather than the link. A procedure printed as
the route out of an exit-6 refusal has to work on the object that produced the
refusal, so rev-11 prints exactly one removal — `rm -rf --` against the single
validated repo-relative managed path, with the `--` terminator, no wildcard and
no recursion beyond that path — and tells the operator plainly to stop and use
kind-appropriate tooling if they want the object preserved. The destructive
warning is printed **above** the command, not after it (§9.3.1, §10.7,
PIB-506, PIB-507, PIB-547).

The rerun is not a new mechanism: it is exactly step 2 of the per-hash recovery
above (absent blob → treat the removal as already complete) followed by step 4
(CAS-publish the tombstone), and it is terminal at exit 0 `recovered` like every
other pending-hash recovery (§7.11). The `retry` field carries that same
sanitized command with `retry_cwd: "workspace-root"`, so the rule that no report
echoes an inherited absolute `--path` holds here too (PIB-497, PIB-507).

**Index divergence** — the index stopped strict-decoding under an external write
after the first mutation. No blob is at fault, so **no blob is named and no
removal of any kind is offered**; the pending hash is known from the strict
preimage this run captured before its
first mutation, and is reported. The route is to restore `index.json` to bytes
that strict-decode — from the operator's own version control or backup, which is
the only place those bytes exist — and then rerun the same sanitized command.
tpatch will not synthesize an index it cannot decode, and the message says that
rather than offering a repair it cannot perform. Removing `index.json` is
**not** offered: it would discard every generation record in the archive, not
just this hash. **This form is deliberately kept separate from the blob form
above and shares none of its commands**: the blob form's `rm -rf --` names a
managed blob path and is type-total over what can be at it, while this form
names no path to remove at all. A message that mixed the two would offer index
deletion under a destructive-removal heading, which is the one thing this route
forbids (PIB-506, PIB-547).

Both forms name only repo-relative paths, state their destructive cost, and
carry the Git-history caveat, exactly as §9.6.2 requires of every statement
about removing archived bytes. Between them they are total over this code's
sub-populations, so this exit-6 code — like every other — has exactly one
applicable route (PIB-506, PIB-507, PIB-508).

This detects pre-CAS changes; it does **not** make CAS→rename/remove atomic. An
editor/Git/other process can replace the index or blob in the final syscall
window and have bytes overwritten/removed, and — separately — can replace the
object at a managed blob path in the gap between the pre-removal revalidation
and the unlink, which the unlink cannot condition on (§9.7.2's window table).
Both residuals are the same disclosed
content race as canonical publication; the workspace lock excludes tpatch
mutators only, not external writers, and no shipped message claims either window
is closed. A crash after a pending rename is always
resumed by the specified per-hash next action, never guessed or restored. A
crash before that rename leaves the old index. Injection covers detected
pre-CAS edits, every pending/recovery crash point, all three partial-outcome
branches, the divergent-evidence procedure, both tombstone-beside-blob
classifications with their `--orphans --yes` and confirmed `--blob <h> --yes`
repairs, the corrupt-object route, the global claim and its five
external-write windows, the selector-independent global scan and its
pending-recovery exception,
the compound pending-plus-residue ladder, the multi-instance repair classes, and
the disclosed post-CAS
final-syscall race (PIB-405, PIB-421,
PIB-422, PIB-429, PIB-430, PIB-443, PIB-447, PIB-465…PIB-468, PIB-489…PIB-491,
PIB-497, PIB-498, PIB-506…PIB-508, PIB-515…PIB-518,
PIB-521…PIB-524, PIB-527, PIB-528, PIB-531…PIB-535, PIB-539…PIB-551).

#### 9.7.3 Orphans and accidental-secret remediation

An orphan is a regular blob whose content hash is **globally unreferenced** by
the strict current index: no reference to it, in any generation, is retained or
removal-pending (§9.3). That covers a blob the index never mentions and a blob
every one of whose references is tombstoned, and it excludes every blob whose
hash is still live. List,
doctor and reports name it; `purge --orphans --yes` removes only validated
orphans and rewrites no index. Operator/Git deletion of a referenced blob is
instead a dangling-reference refusal (X11), not an orphan.

**A tombstoned reference whose named blob is present is an orphan only when the
hash is unreferenced everywhere.** The tombstone's `blob` locator is empty, so
*that* reference names nothing; whether the **file** is unreferenced is a global
question, and it is answered over every generation before anything is called an
orphan. When no reference to the hash is retained or removal-pending, the file
is unreferenced physical residue. X11 refuses exit 3
`archive-index-storage-inconsistent` on discovery, writing nothing, and names
this one repair, character for character, in the refusal, in `list` and in
`doctor`:

```text
tpatch feature intent-archive purge <slug> --orphans --yes
```

run from the workspace root. That invocation is admitted past the X11 refusal
after the strict X1–X10 wire decode, validates each file as a regular blob whose
bytes hash to its name, removes it, and rewrites no index — the tombstone
already says what is true, so nothing about the archive's meaning changes. **It
removes every unreferenced residue in the archive, not one**: the admission is
for the residue *class*, and one invocation is expected to clear all of its
instances, so an archive holding three of them is repaired in one run rather
than bricked because each instance disqualifies the others (§9.3.1, PIB-548,
PIB-549). A
non-regular or hash-wrong file at that path is not an orphan and keeps its
`archive-blob-corrupt` exit-3 refusal, whose own route is §9.3.1's type-total
removal. No other repair is offered, and the
archive-divergence procedure of §9.7.2 is never named for it, because there is
no pending reference for that procedure to finalize (§9.3.1, PIB-521, PIB-522,
PIB-523, PIB-524).

**When the hash is still live, the same observation is not an orphan and
`--orphans` must not touch it.** If any other reference to that hash — in the
same generation or another — is **retained**, the blob is that
reference's live storage. Removing it would turn a retained reference into a
dangling one, which is exactly the loss `--orphans` promises never to cause. The
state is a mixed tombstone/live-reference global-availability inconsistency,
X11 refuses exit 3 `archive-index-storage-inconsistent` writing nothing, and the
one repair named character for character by the refusal, by `list` and by
`doctor` is the ordinary confirmed global-hash purge:

```text
tpatch feature intent-archive purge <slug> --blob <hash> --yes
```

run from the workspace root. It selects **every** reference to that hash, claims
them all, removes the blob and tombstones them all, which
makes the index's global availability claim true again. `--orphans --yes` never
removes that blob, and no report ever names `--orphans` for this state
(§9.3.1 subcase B, PIB-531, PIB-532, PIB-533, PIB-535). If instead any
reference to that hash is **removal-pending**, the hash is owned by a purge
transaction in flight (§9.3): the observation is reported as `recovery-pending`
and routed to that transaction, and neither subcase's repair is named
(PIB-539).

For an accidentally archived secret, use:

```text
tpatch feature intent-archive list <slug>
tpatch feature intent-archive purge <slug> --blob <hash> --yes
```

The second command tombstones every reference to the hash. A committed blob
remains in Git history; removing history is a Git operation tpatch does not
perform. An untracked archive can also disappear through clone or `git clean`;
that is loss, not a Git-independent recovery guarantee.

For a missing retained blob, `list`, doctor and the refusal report name the
**same single literal command**, character for character:

```text
tpatch feature intent-archive purge <slug> --blob <hash> --yes
```

It tombstones every reference to that hash after confirming the blob is still
absent, and performs no removal. No other repair is offered, suggested or
implied: rehydration is not a dangling-reference remedy, because X11 refuses
every ordinary mutation that could have produced the replacement bytes
(§9.3.1). After the repair, an ordinary later regeneration that reproduces the
tombstoned generation rehydrates it through §9.3's global path. No ordinary
generation/archive mutation guesses which repair the operator intended
(PIB-428, PIB-444, PIB-448, PIB-457…PIB-460).

**A blob that is present but unidentifiable is neither dangling nor an
orphan.** A non-regular file — a symlink, a directory, a FIFO or a device node —
or one whose bytes do not hash to its name, under
a **retained** reference is `archive-blob-corrupt` (exit 3), rendered `corrupt`
by `list` and reported by `doctor`, and its route is §9.3.1's procedure: one
type-total `rm -rf --` of the exact managed blob path, printed under an explicit
destructive warning, then the confirmed `--blob <hash> --yes` above — which by
then is the
ordinary dangling repair — or restore the exact correct blob and retry. **No
preservation copy is promised**, because no single copy command is correct for
every kind of object that can occupy that path; an operator who wants the object
is told to stop and use kind-appropriate tooling (§9.3.1, PIB-547). It is
never an orphan, because the hash is live; `--orphans --yes` never touches it;
and the per-orphan identity check of §9.7.2, which yields the same code over the
globally **unreferenced** set, routes to the same type-total removal, after
which the hash is simply unreferenced with no file (PIB-428, PIB-543).

### 9.8 Why the archive is not provenance — stated so it cannot drift

| Question a provenance record answers | Does the archive answer it? |
|---|---|
| Who or what authored these bytes? | **No.** Nothing is recorded about the author. |
| Was this artifact produced by Path A or Path B? | **No.** `mode` names the *tpatch invocation mode that replaced it*, not the origin of the replaced bytes. A hand-authored file replaced by `--regenerate` records `mode: "regenerate"` — a fact about the replacing act. |
| Which model/provider produced them? | **No.** No provider, model, endpoint or configuration is recorded. |
| Is the current `spec.md` provider-generated? | **No.** The archive says nothing about the *current* file. |
| Have these exact bytes existed at this path before? | **Yes** — and that is the only question it answers, and only until they are purged. |

Consequently:

- `prepare --check` continues to report the constant `provenance: unknown` for
  every artifact, and **must not** consult the archive (PIB-140, source-scanned
  by PIB-141).
- No mutating report field may assert Path A vs Path B for a feature (PIB-142).
- The forbidden-inference list of the accepted contract
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920`) is extended
  by exactly one entry — **the intent archive** — and PIB-143 asserts that
  entry exists in the shipped source.
- `status.json.notes` remains a **last-transition hint, not provenance**
  (§12.3.2), and the archive does not upgrade it into one.
- Any future PRD that wants to answer the first four rows must write the
  provenance ADR first. This PRD does not, and ADR-035 D9 says so normatively.

## 10. CLI output, exit codes and precedence

### 10.1 Stream routing

Composed with the **real** root error printer, which emits `error: %v` for
every non-nil `RunE` error (`internal/cli/cobra.go:33-39`) before
`exitCodeFor` maps the code (`internal/cli/cobra.go:43-52`). This PRD inherits
that composition from the accepted contract
(`docs/prds/PRD-artifact-validation-and-provenance.md:2367-2419`) and does not
re-litigate it.

| Flags | stdout | stderr |
|---|---|---|
| (none) | human report + progress lines | one `error:` line on every nonzero exit |
| `--json` | the JSON report, one document, trailing newline | one `error:` line on every nonzero exit |
| `--quiet` | one summary line only | one `error:` line on every nonzero exit |
| `--json --quiet` | the JSON report only | one `error:` line on every nonzero exit; **empty only on exit 0** |

**Progress vs final report.** Progress lines ("[1/3] Generating analysis…")
are written to **stderr**, never stdout, and are suppressed entirely under
`--json` or `--quiet`. This keeps stdout a single parseable document under
`--json`, which the shipped `next --format harness-json` convention already
assumes (`internal/cli/phase2.go:381-384`). The existing phase commands print
progress to stdout (`internal/cli/cobra.go:621-626`); this command deliberately
does not, and §12.6 D5 enumerates that as a *new-surface* choice, not a change
to those commands. PIB-170, PIB-171.

### 10.2 JSON report (v1)

`schema_version: 1`. Fixed key order by struct declaration order. No Go map.
This is a **different schema** from `--check`'s: `command` distinguishes them
and consumers must switch on it (PIB-172).

```json
{
  "schema_version": 1,
  "command": "prepare",
  "mode": "regenerate",
  "dry_run": false,
  "slug": "fix-model-id-translation",
  "outcome": "published",
  "action": "regenerate",
  "feature_state": "defined",
  "disclaimer": "Structural presence only. This report does not certify semantic quality.",
  "artifacts": [
    {
      "id": "analysis",
      "path": ".tpatch/features/fix-model-id-translation/analysis.md",
      "role": "required",
      "disposition": "regenerated",
      "generator": "provider",
      "archived_blob": "a91e…"
    },
    {
      "id": "spec",
      "path": ".tpatch/features/fix-model-id-translation/spec.md",
      "role": "required",
      "disposition": "regenerated",
      "generator": "heuristic",
      "archived_blob": "77bd…"
    },
    {
      "id": "exploration",
      "path": ".tpatch/features/fix-model-id-translation/exploration.md",
      "role": "required",
      "disposition": "regenerated",
      "generator": "heuristic",
      "archived_blob": ""
    },
    {
      "id": "analysis_sidecar",
      "path": ".tpatch/features/fix-model-id-translation/artifacts/analysis.json",
      "role": "optional",
      "disposition": "regenerated",
      "generator": "provider",
      "archived_blob": "c40a…"
    }
  ],
  "archive": {
    "generation_id": "3f0c…",
    "blobs_dir": ".tpatch/features/fix-model-id-translation/artifacts/intent-archive/blobs"
  },
  "orphan_blobs": [],
  "advisories": [
    {
      "code": "regenerate-heuristic-allowed",
      "artifact_id": "",
      "message": "--allow-heuristic was passed, so at least one artifact was replaced by heuristic output."
    },
    {
      "code": "provider-fallback-heuristic",
      "artifact_id": "spec",
      "message": "The provider call failed and the heuristic generator was used instead. Re-run after fixing the provider to replace it."
    }
  ]
}
```

The example is a `--regenerate --allow-heuristic` run: without that flag a
provider failure would have refused before publication (§11.3.2), so a
`regenerate` report can only carry `generator: "heuristic"` alongside
`regenerate-heuristic-allowed` (PIB-372).

Closed vocabularies:

| Field | Closed set |
|---|---|
| `mode` | `generate`, `manual`, `regenerate`, `abandon` (`check` never emits this schema) |
| `outcome` | `published`, `no-op`, `planned`, `refused`, `rolled-back`, `recovered`, `recovery-refused`, `recovery-required`, `abandoned`, `abandon-planned`, `purge-partial` |
| `action` | `none`, `adopt`, `complete`, `regenerate`, `abandon` |
| `disposition` (per artifact) | `preserved`, `generated`, `regenerated`, `untouched`, `absent-optional` |
| `generator` | `provider`, `heuristic`, `` (empty for anything not generated this run) |
| `execution_preflight` | `not_evaluated` (emitted **only** in a dry-run report; §6.4) |
| `recovery.kind` | `journal-undo`, `archive-purge-finalize` (emitted **only** on the `recovered` outcome) |
| `purge_progress.resume` | `pending-recovery-then-completion`, `completion-only`, `orphan-scan` (emitted **only** on the `purge-partial` outcome; §9.7.2) |
| `pending_purge.selector` | `blob`, `generation`, `all`, `orphans` (emitted **only** on the `recovery-required` outcome; §9.7) |
| `retry_cwd` | `workspace-root` (emitted wherever a `retry` command is emitted; §9.7.2) |
| `advisories[].code` | the seventeen codes of §10.3 |
| `refusal.code` | the closed refusal catalog of §10.4, asserted complete by PIB-228 |
| `orphan_blobs[]` | zero or more `^[0-9a-f]{64}$` hashes; present on every outcome that leaves one (§7.9, §9.7.3) |

**Dry-run shape.** A `--dry-run` report additionally carries `dry_run: true`,
the planned `actions` array and the closed field
`execution_preflight: "not_evaluated"`, plus the verbatim plan-only sentence of
§6.4. Neither field appears in a non-dry-run report, and no dry-run report
carries `archive`, a `generator` value or a blob hash (PIB-077, PIB-462).

**Partial-purge shape.** The archive `purge` report is the same schema with
`command: "feature intent-archive purge"`. On the `purge-partial` outcome
(§9.7.2, exit 5) it carries a `purge_progress` object with exactly
`completed_hashes[]`, an optional `pending_hash`, `remaining_hashes[]`, the
closed `resume` discriminator, the sanitized `retry` command line, `retry_cwd`
and the one-line consistent-`state` statement. `pending_hash` is present **if
and only if** `resume` is `pending-recovery-then-completion`; on
`completion-only` and `orphan-scan` it is absent and no `recovered` outcome is
promised for the retry.
Those fields appear on no other outcome, and PIB-466, PIB-516, PIB-517 and
PIB-518 assert them against a real interrupted purge of each branch.

**Pending-purge preview shape.** A `feature intent-archive purge` **preview**
(no `--yes`) that finds no journal and one or more removal-pending hashes
reports the closed outcome token `recovery-required` — a preview that recovered
nothing, planned nothing and is not a refusal, so none of `planned`, `refused`
or `recovered` can carry it truthfully. `action` is `none`. The report carries
exactly one extra object, `pending_purge`, with exactly these fields in this
order:

```json
{
  "outcome": "recovery-required",
  "action": "none",
  "pending_purge": {
    "recovery_required": true,
    "pending_hashes": [
      {
        "hash": "b12d…",
        "blob": ".tpatch/features/fix-model-id-translation/artifacts/intent-archive/blobs/b12d….blob",
        "index": ".tpatch/features/fix-model-id-translation/artifacts/intent-archive/index.json",
        "plan": "claim every reference to it, remove the blob if it is present, then tombstone every reference to it"
      }
    ],
    "selector": "all",
    "retry": "tpatch feature intent-archive purge fix-model-id-translation --all --yes",
    "retry_cwd": "workspace-root"
  }
}
```

Every field is normative:

- `recovery_required` is the constant `true`; the object is emitted on no other
  outcome, so a consumer switching on `outcome` and a consumer switching on the
  field agree.
- `pending_hashes[]` is always present and never null (ADR-033 D11), is sorted
  lexically by `hash`, and holds one object per removal-pending hash. `hash`
  matches `^[0-9a-f]{64}$`. `blob` and `index` are **repo-relative** paths under
  the feature's `artifacts/intent-archive/` directory and are the only paths the
  object carries; no absolute path, no symlink target and no path outside that
  directory may appear (§10.6, PIB-497). `plan` is the fixed per-hash sentence
  above, identical for every entry, because the recovery's next action is
  decided from the blob observation at recovery time and not at preview time.
- `selector` is the closed field naming which selector the operator passed. It
  exists so a consumer can verify the retry without parsing it.
- `retry` is the operator's own command with `--yes` appended and **no**
  root-selection argv — the same sanitization every retry in this PRD obeys
  (§9.7.2) — and it preserves the operator's selector exactly, including their
  own hashes or generation ids.
- `retry_cwd` is the closed literal `workspace-root`.

The human rendering of §9.7 carries the same hashes, the same two repo-relative
paths, the same plan sentence and the same retry line, under the one verbatim
`Run this again from the same workspace root:` heading every retry in this
cluster uses — emitted at column 0, with no leading or trailing whitespace and
nothing else on its line (§9.7.2) — so the
two surfaces are checkable against each other field by field rather than
described twice. The report carries no `recovery` object, no `purge_progress`,
no `archive` and no blob hash outside `pending_hashes[]`; the invocation
acquires no lock and writes nothing (PIB-498, PIB-515, PIB-529, PIB-530).

**Recovered shape.** A terminal recovery (§7.11) reports `outcome: "recovered"`,
`action: "none"` and a `recovery` object with exactly these fields, in this
order:

```json
{
  "outcome": "recovered",
  "action": "none",
  "recovery": {
    "kind": "journal-undo",
    "restored_entries": [
      ".tpatch/features/fix-model-id-translation/spec.md",
      ".tpatch/features/fix-model-id-translation/status.json"
    ],
    "finalized_hashes": [],
    "retry": "tpatch prepare fix-model-id-translation --regenerate",
    "retry_cwd": "workspace-root"
  }
}
```

`kind` is closed at `journal-undo` and `archive-purge-finalize`.
`restored_entries[]` holds repo-relative paths and is empty for a
`archive-purge-finalize` recovery; `finalized_hashes[]` holds
`^[0-9a-f]{64}$` hashes and is empty for a `journal-undo` recovery. Both arrays
are always present, never null (ADR-033 D11). `retry` is the sanitized
equivalent of the command the operator ran and never carries a root-selection
argv; `retry_cwd` is the closed literal `workspace-root` (§9.7.2). The
`recovery` object appears on no other outcome, `artifacts` carries the
dispositions the recovery restored to rather than a plan, and `archive` is
omitted (PIB-483, PIB-486, PIB-497, PIB-498).

**`generator` is a statement about this process, not about the file's
history.** It is emitted transiently in the mutating report and is never
persisted, under that key name or any synonym, into `status.json`, the sidecar,
the archive index, a blob, the journal or any other tracked or local file this
command writes. §9.8 governs why it is not provenance.

**The guard for that claim is schema-scoped, not byte-scoped.** rev-4 asked for
a scan proving the token `generator` appears in no `.tpatch/` byte after a run.
That is not satisfiable and never was: `analysis.md`, `spec.md` and
`exploration.md` are canonical prose that a provider or a human may legitimately
write the word "generator" into, and a guard that failed on it would forbid
ordinary English. The shipped guard therefore walks (a) the **declared keys** of
every wire struct this PRD adds or writes — the report schema, `status.json`'s
field set, the sidecar, `index.json` and the journal — and (b) the **enumerated
persistence sinks** of §13.5, asserting that no key named `generator` (or an
enumerated synonym such as `generated_by`, `producer` or `authored_by`) is
declared or written there. A fixture whose canonical artifact prose contains the
word must **pass**; a fixture that adds a `generator` key to any persisted
schema must **fail** (PIB-144, PIB-477).

**Refusal shape.** On any refusal, `outcome` is `refused`, `artifacts` carries
the per-artifact dispositions the plan *would* have had (or `[]` when the
refusal happens before inspection, e.g. workspace or slug), `archive` is
omitted, `orphan_blobs` lists every blob this attempt left behind (usually
empty), and a `refusal` object carries a closed `code`, a message and a
self-contained remediation (PIB-173, PIB-174, PIB-316).

**Abandon shape.** In `abandon` mode the report carries `mode: "abandon"`,
`outcome: "abandon-planned"` (preview) or `"abandoned"` (`--yes`), an
`abandoned` object naming the evidence directory and the control files moved
into it, `artifacts: []` — because the mode inspects no artifact and asserts
nothing about them — and `archive` omitted (PIB-273, PIB-366). On the
`no-pending-transaction` refusal that fires when only prior `abandoned-<12hex>/`
residue remains (§6.6 rule 7), `outcome` is `refused` and the same `abandoned`
object carries an `existing[]` array of repo-relative residue directories and
nothing else (PIB-499).

### 10.3 Advisory catalog (closed, seventeen codes)

| Code | Fires when | Says |
|---|---|---|
| `provider-not-configured` | default mode, no provider configured or reachable | the heuristic generator was used; how to configure one |
| `provider-fallback-heuristic` | default mode (or `--regenerate --allow-heuristic`), a provider call failed and the heuristic generator produced the artifact | which artifact; that re-running replaces it |
| `provider-deadline-heuristic` | a **deadline** (per-phase or total) rather than a provider error caused the fallback | which artifact, and which of the two deadlines expired |
| `provider-deadline-cascade` | the **total** deadline expired and therefore two or more artifacts fell back for the same reason | how many artifacts, and that one expiry — not three independent failures — caused it (§11.5) |
| `regenerate-heuristic-allowed` | `--regenerate --allow-heuristic` was used **and** at least one artifact fell back | that a hand-authored bundle was replaced by heuristic output at the operator's explicit request |
| `allow-heuristic-redundant` | `--allow-heuristic` was passed in default mode | that fallback is already the default there and the flag changed nothing |
| `analysis-preserved-sidecar-untouched` | `analysis.md` preserved, sidecar absent | the sidecar is not synthesized for a preserved analysis, and its absence is not a defect |
| `bundle-untracked-in-git` | `--regenerate`, `.git` present, `.tpatch/` not tracked (`internal/cli/cobra.go:3405-3407`) | the archive is at risk of loss on clone **and `git clean`** until committed (§9.6.2) |
| `workspace-not-git` | G1 established a non-worktree, so G2–G4 were inapplicable and skipped (§7.13) | that only G1 ran and no Git recovery route exists here |
| `archive-blob-reused` | a replaced artifact's content already existed as a blob | zero new bytes were written for it |
| `archive-generation-duplicate` | the computed `generation_id` already exists in the index | no index entry was appended; the archive is a set |
| `archive-orphan-blobs` | orphan blobs exist for this slug after this invocation, whatever its outcome | how many, and the exact `purge --orphans` command |
| `staging-retained` | a failure left a staging tree | its repo-relative path; that the next successful run removes it |
| `recovered-prior-transaction` | a terminal recovery completed (§7.11): a pending journal was undone/cleared, or pending archive hashes were finalized | which entries were restored or which hashes were finalized, that the requested operation was **not** performed, and the sanitized retry that performs it |
| `features-index-refresh-failed` | the best-effort `FEATURES.md` refresh returned an error (§12.3.1) | that `status.json` is authoritative and the next transition retries the index |
| `feature-state-below-defined` | the source state was `requested` or `analyzed` | the transition that was performed |
| `heuristic-mode-recorded-in-sidecar` | the sidecar was generated in heuristic mode | that `heuristic_mode: true` is set in the sidecar, exactly as `RunAnalysis` does today (`internal/workflow/workflow.go:25,208`) |

Advisory selection is a **total function of observed state**, never of artifact
id: every advisory's precondition is a predicate over the inspection result and
the transaction outcome, and PIB-175 asserts no advisory can contradict its own
artifact row. PIB-227 asserts the shipped catalog is exactly these seventeen
codes and that each is reachable by a named fixture.

### 10.4 Exit codes

Per-command contract, per `SPEC.md:137`.

| Code | Meaning | Report emitted? | Wrote anything? |
|---|---|---|---|
| `0` | success: published, no-op, plan OK, preview of an abandon/purge, a pending-purge preview's `recovery-required` (§9.7), a terminal `recovered` outcome (§7.11), or `--check` ready | yes | only on `published`, on a terminal `recovered`, and on `--yes` abandon/purge |
| `1` | generic CLI/parse error (arity, unknown flag, mutually exclusive flags, missing purge scope), or an unexpected internal error | no (parse) / yes (internal) | no |
| `2` | **not-ready / not-coherent refusal**: `--manual` on an incomplete bundle; default mode on a `present-empty` required artifact; an incoherent bundle gap (§6.1.2); staged-output validation failure | yes | no |
| `3` | **cannot-act refusal**, four documented populations distinguished by `refusal.code`: (a) *indeterminate* — workspace not initialized, feature not found, unsafe slug, `status.json` malformed/unreadable, an artifact in an unsafe/unstable state, archive or index corruption, unreferenced physical residue under a tombstone, the mixed tombstone/live-reference state beside a live blob and a present but unidentifiable blob under a retained reference (§9.3.1), local-lane gate failure or unverifiability, unsupported platform, unsupported lock filesystem, an authority that could not be established at all (root open, directory open or a non-contention `flock` failure, §7.4.1), `request.md` unreadable; (b) *lifecycle-state* — the source state does not permit preparation (§12.2); (c) *transient* — `transaction-in-progress` (a live sibling holds the lock), `recovery-pending` (`--dry-run` or `purge` with a pending journal, and a normal mutating `prepare` that observes a removal-pending archive index, §7.8 step 5); (d) *policy* — `provider-required-for-regenerate`, `archive-content-refused-sensitive` | yes | no |
| `4` | **retired.** The reserved-surface population (`prepare` without `--check`) no longer exists, and no new population is bound to it. `prepare` never exits 4. | — | — |
| `5` | **transaction aborted or archive purge partially applied**, two populations, distinguished by `refusal.code`/`outcome`: (a) *publication abort* — revalidation or per-entry CAS mismatch, generation failure after staging began, or a rename failure that was **successfully rolled back**; no canonical artifact, no `status.json` and no index entry changed, and orphan blobs may remain and are listed (§7.9). (b) *`archive-purge-partial`* — a purge that had already begun its deterministic per-hash mutations hit a later I/O or external-change failure; the index has advanced but decodes, is consistent and claims no absent bytes, and re-running the reported **sanitized equivalent command** resumes it. How many runs that takes is stated by the report's `resume` field and is not promised uniformly: with a pending hash it is one `recovered` run and then one completing run; with the failure exactly between hashes, or on `--orphans`, it is a single completing run and **no** `recovered` outcome (§9.7.2). Both are retryable without human judgement. | yes | (a) possibly additive orphan blobs, fully enumerated; (b) the enumerated per-hash index/blob progress |
| `6` | **manual intervention required**: an undo CAS refused, rollback failed, recovery found divergent evidence, a journal failed a J1–J10 bind, the final verification found post-publication divergence, or purge evidence became divergent/unsafe under a pending reference. Everything is preserved, and the message names the **one** route that applies to that population — §6.6's abandon route for the journal and publication codes, §9.7.2's archive procedure for `archive-purge-evidence-divergent`. | yes | possibly a partial publication, fully described |

**Exit 3 keeps its zero-write promise, absolutely.** Every exit-3 population is
reached before the invocation's first byte of mutation, including the whole
purge preflight of §9.7.2. The statement carries **no** post-recovery
qualification and needs none: a successful recovery is terminal at exit 0
(§7.11), so no invocation can both write during recovery and then return exit 3.
PIB-469 asserts this over the entire exit-3 population with a whole-tree byte
snapshot rather than per-case reasoning, and its sensitivity fixture — a code
path that writes and then returns exit 3, including one that recovers and then
falls through to a later gate — must fail the guard (PIB-469, PIB-487).

`0`/`2`/`3` keep the meanings the accepted `--check` contract already binds
(`docs/prds/PRD-artifact-validation-and-provenance.md:1981-1987`), so the same
condition yields the same code in both halves of the verb. Population (b) of
exit `3` additionally matches the repository's shipped state-refusal
convention: `reject`, `reopen` and `refuseIfUnappliedState` all bind
state-transition refusal to `3` (`internal/cli/reject.go:45-48,68-70`;
`internal/cli/feature_unapply.go:464-473`; `SPEC.md:148`). Collapsing lifecycle
refusal into `3` rather than inventing a fifth code is what lets `prepare`
reuse the shipped `refuseIfUnappliedState` helper unchanged. `5` and `6` are
new and have no read-mode population.

**Every exit-6 population has exactly one applicable, named route out.** rev-6
said "exit 6 is never terminal" and then named a single route —
`--abandon-transaction` — for all ten codes. That was true of nine of them and
false of the tenth, because abandon touches no archive index and no blob (§6.6
rules 8 and 10) and therefore cannot consume archive divergence. rev-7
partitions the population instead of widening one command's promise:

| Exit-6 codes | The one route | Where it is defined |
|---|---|---|
| `undo-cas-mismatch`, `recovery-divergent`, `journal-corrupt`, `journal-version-mismatch`, `journal-foreign`, `journal-path-escape`, `journal-forged`, `post-publication-divergence`, `workspace-root-replaced-after-publication` | `tpatch prepare <slug> --abandon-transaction` — and, where the environment denies that mode, the named repo-relative manual removal | §6.6 |
| `archive-purge-evidence-divergent` | the repo-relative archive procedure: for the **blob** form, one type-total `rm -rf --` of the divergent managed `blobs/<hash>.blob` under the pending reference, printed under an explicit destructive warning and with no copy command promised; for the **index** form, restore an `index.json` that no longer decodes and remove nothing; then rerun the sanitized purge, whose pending+absent recovery terminally tombstones | §9.7.2 |

Every message in the first population names, in this order: the
divergent entry, the file that holds its prior bytes (an archive blob or a
journal-directory preimage), and the exact command
`tpatch prepare <slug> --abandon-transaction` to unblock the slug without
touching any canonical file (§6.6). Every message in the second names the
pending hash, the repo-relative managed blob and index paths, the destructive
warning, the type-total removal step (blob form) or the restoration step (index
form), and the sanitized rerun with
`retry_cwd: "workspace-root"` — and it **must not** name abandon, because
abandon cannot act on it. PIB-362 asserts the first population's three elements,
PIB-506 asserts the second's, and PIB-508 asserts the partition itself: each
code maps to exactly one route, and a fixture that routes archive divergence to
abandon fails the guard. PIB-363 and PIB-507 assert that after the applicable
route runs, the affected slug or archive reaches exit 0 again.

The abandon half of that partition is **qualified, not unconditional**, and the
qualification is itself a route rather than a caveat.
`--abandon-transaction` needs the mutating
platform, a non-denied root filesystem and the exclusive root lock (§6.6 rule
1); it does **not** need Git (§6.6 rule 2), and it does **not** need the feature
directory or a readable `status.json` (§6.6 rule 3). Where the mutating
environment is unavailable — `prepare-unsupported-platform`,
`lock-filesystem-unsupported`, `directory-flock-unavailable` — the refusal names
the repo-relative lane
`.tpatch/local/intent-prepare/<slug>/` and the last-resort manual removal that
unblocks the slug without touching a canonical file, together with what that
costs (§6.6). Lock **contention** is the one stop that offers no manual removal,
because a live holder may be mid-transaction; its route is wait and retry
(§6.6's gate table, PIB-512). So the accurate statement, and the only one any
shipped string or document in this cluster is permitted to make, is: **every
exit-6 population has a named, executable route out — the abandon mode where the
environment supports mutation, the named manual procedure where it does not, and
the archive procedure for archive divergence.** PIB-496 is the
guard that fails an unqualified "always recoverable" sentence anywhere in this
cluster's strings or docs.

### 10.4.1 Closed refusal catalog — code, exit, human and JSON shape

Every reachable refusal uses exactly one stable code below. Human output begins
`prepare <slug>: <mode> refused <code>` (or the archive command equivalent) and
names the listed remediation without an absolute path; JSON carries the same
literal in `refusal.code`, `refusal.message`, and `refusal.remediation`.

| Code(s) | Exit | Remediation class |
|---|---:|---|
| `slug-unsafe`, `workspace-not-initialized`, `workspace-unsupported-platform`, `prepare-unsupported-platform`, `lock-filesystem-unsupported`, `directory-flock-unavailable`, `local-lane-not-ignored`, `local-lane-unverifiable`, `feature-not-found`, `status-malformed`, `status-unreadable`, `request-unreadable`, `artifact-unsafe`, `artifact-unstable`, `state-refused`, `abandon-evidence-unsafe` | 3 | fix the named workspace/input/state condition; a denied/failed root filesystem says “move workspace to a supported local filesystem”; unsafe abandon evidence names the exact lane entry whose kind or containment failed, and moves nothing. **When `prepare-unsupported-platform`, `lock-filesystem-unsupported` or `directory-flock-unavailable` is returned while journal, preimage or staging evidence exists for the slug, the message additionally names the repo-relative lane `.tpatch/local/intent-prepare/<slug>/` and the last-resort manual removal, so the exit-6 escape stays executable there too (§6.6's pre-abandon gate table, rows 5, 6 and 8)**. `feature-not-found`, `status-malformed` and `status-unreadable` are **unreachable in `abandon` mode**, which never reads the feature directory (§6.6 rule 3, §10.5 step 6) |
| `transaction-in-progress`, `recovery-pending` | 3 | for `transaction-in-progress`: the workspace mutation authority is held, the holder's identity is unknowable, and the safe action is to wait and retry — no stronger claim is made anywhere (§12.5), and **no manual removal is offered even in `abandon` mode with evidence present**, because the evidence may be a live holder's undo journal (§6.6's gate table row 7, PIB-512). `recovery-pending` has **two** populations and each names exactly one route. *Pending journal* (`--dry-run`, and `intent-archive purge` in both its preview and `--yes` forms): run a mutating `prepare <slug>`, which recovers the journal and reports `recovered` (§7.11), or `tpatch prepare <slug> --abandon-transaction --yes`; the refusing command never recovers, decodes or moves the journal itself. *Pending archive index* (a **normal mutating `prepare`** — `generate`, `manual`, `regenerate` — that observes removal-pending references, §7.8 step 5): run the sanitized `tpatch feature intent-archive purge <slug> --all --yes` from the workspace root, or `--blob <h> --yes` where every pending reference shares one hash `h`; the refusing `prepare` never finalizes a pending hash, and `retry_cwd: "workspace-root"` accompanies the named command. This population is defined by **global hash ownership**: any removal-pending reference makes the purge transaction the owner of its content hash, ownership outranks every other classification of that hash, and it fires whether or not the refusing mode would have touched that hash — `--manual`, which writes no archive at all, is refused by it (§9.3, §6.2, PIB-525, PIB-526, PIB-538, PIB-539) |
| `provider-required-for-regenerate`, `archive-content-refused-sensitive` | 3 | configure provider/use explicit heuristic opt-in, or remove sensitive material and retry |
| `archive-index-corrupt`, `archive-index-version-unsupported`, `archive-index-foreign`, `archive-index-path-escape`, `archive-index-generation-mismatch`, `archive-generation-id-collision`, `archive-blob-corrupt`, `archive-blob-dangling`, `archive-index-storage-inconsistent`, `archive-blob-shared`, `archive-purge-index-changed` | 3 | preserve bytes; upgrade/inspect; for a dangling `h`, the one named repair is the literal `tpatch feature intent-archive purge <slug> --blob <h> --yes` (or an applicable `--all --yes`), which tombstones every reference to `h` and removes nothing. `archive-index-storage-inconsistent` has **three** sub-populations with three different routes, decided from the hash's **global** liveness (§9.3): an unsafe/wrong blob under a **pending** reference preserves the evidence and is resolved by §9.7.2's archive procedure; a **tombstoned reference whose named blob is present while the hash is globally unreferenced** is physical residue whose one named repair is the literal `tpatch feature intent-archive purge <slug> --orphans --yes` run from the workspace root; and the **same tombstone beside a blob whose hash is still live** is a mixed tombstone/live-reference inconsistency whose one named repair is the literal `tpatch feature intent-archive purge <slug> --blob <h> --yes` run from the workspace root, `--orphans` being forbidden from touching a live blob (§9.3.1, §9.7.3, PIB-521, PIB-522, PIB-531, PIB-532). `archive-blob-corrupt` likewise has **two** sub-populations, and both take the same type-total removal: over the globally **unreferenced** set it is `--orphans`' per-blob identity refusal, whose route is one `rm -rf -- <managed blob path>` after which the hash is simply unreferenced with no file and nothing further is required; under a **retained** reference it is a present but unidentifiable live blob, whose one named route is §9.3.1's repo-relative procedure — an explicit destructive warning, one `rm -rf --` of the exact managed blob path, then the confirmed `tpatch feature intent-archive purge <slug> --blob <h> --yes` that tombstones every now-dangling reference, or restore the exact correct blob and retry — stated with its destructive cost and the Git-history caveat, and with **no** generic preservation copy, because no single copy command is correct for a regular file, a symlink, a directory, a FIFO and a device node alike (§9.3.1, §9.7.3, §10.7, PIB-428, PIB-543, PIB-547). Every code in this row is refused over a **global** X11 scan of the whole index, so an inconsistency on a hash the requested selector does not touch still refuses it zero-write and names that hash's own repair; an admitted repair is admitted for its whole **class** and only when the confirmed selection covers every instance of that class, so multiple same-class instances are repaired in one invocation while mixed classes refuse zero-write and render each class's route (§9.3.1, PIB-542, PIB-548, PIB-549) |
| `no-pending-transaction` | 3 | run an admissible mutating operation, or inspect the stated residue; when the only pending state is a removal-pending archive index, the message names the archive route (`feature intent-archive purge <slug> --blob <h> --yes`, or `--all --yes` for more than one pending hash) and **not** a mutating `prepare`, which since rev-8 refuses `recovery-pending` in that state rather than finalizing it (§6.6 rule 8, §7.8 step 5); when the only residue is one or more prior `abandoned-<12hex>/` directories, the message lists them repo-relative, states that they were preserved untouched, and offers the optional `rm -rf` form (§6.6 rule 7); no destructive cleanup is implied or performed |
| `incoherent-bundle-gap`, `artifact-empty-not-overwritten`, `not-ready`, `staged-output-invalid` | 2 | complete/adopt or choose the explicitly named regenerate/manual route |
| `entry-appeared`, `entry-changed`, `status-changed`, `archive-index-changed`, `regenerate-generation-failed`, `workspace-root-changed` | 5 | no canonical overwrite; retry from the observed tree. A missing/changed original workspace path must be restored/reselected; a held root cannot discover a moved pathname |
| `archive-purge-partial` | 5 | the purge is resumable: re-run the reported **sanitized equivalent command** from the workspace root (`retry` plus `retry_cwd`, never an inherited `--path`). What that run does is stated by the report's closed `resume` field: on `pending-recovery-then-completion` it finalizes the pending hash and reports exit 0 `recovered` (terminal, §7.11) and a further run completes the remaining hashes; on `completion-only` and `orphan-scan` a single run completes the work and **no** `recovered` outcome is emitted or promised. The report's `completed_hashes`, `pending_hash` (branch (a) only) and `remaining_hashes` say where it stopped, and the index is consistent in the meantime (§9.7.2) |
| `undo-cas-mismatch`, `recovery-divergent`, `journal-corrupt`, `journal-version-mismatch`, `journal-foreign`, `journal-path-escape`, `journal-forged`, `post-publication-divergence`, `workspace-root-replaced-after-publication` | 6 | preserve evidence and name `--abandon-transaction`; where platform, filesystem or `flock` denies that mode and evidence exists, name the repo-relative lane and the last-resort manual removal instead (§6.6) |
| `archive-purge-evidence-divergent` | 6 | preserve every blob, the index and the pending record, and name the **archive** procedure of §9.7.2 — the pending hash, the repo-relative managed blob and `index.json` paths, an explicit destructive warning, the type-total `rm -rf --` of the divergent managed blob path (blob form) **or** the restoration of an index that no longer strict-decodes (index form, which names no removal at all), and the sanitized rerun with `retry_cwd`, whose pending+absent recovery terminally tombstones — together with its destructive cost and the Git-history caveat. No generic preservation copy is offered, because no one copy command is correct for a regular file, a symlink, a directory, a FIFO and a device node alike (PIB-547). It **must not** name `--abandon-transaction`: that mode touches no index and no blob and cannot consume this evidence (§6.6 rules 8 and 10, PIB-506, PIB-508). It is reachable **only** where a pending reference exists; a tombstoned reference beside a present blob is not this code but exit-3 `archive-index-storage-inconsistent`, with the `--orphans --yes` repair when the hash is globally unreferenced and the confirmed `--blob <h> --yes` repair when it is still live (§9.3.1, PIB-521, PIB-524, PIB-531), and a retained reference beside a present but unidentifiable blob is exit-3 `archive-blob-corrupt` with §9.3.1's own route (PIB-543) |

`archive-index-changed` is publication/rehydration exit 5 only;
`archive-purge-index-changed` is purge exit 3 only (preflight, before the first
write); `archive-purge-partial` and `archive-purge-evidence-divergent` are
`feature intent-archive purge --yes` only — no other command can produce them,
because no other command runs the per-hash machine (§7.11, PIB-526) — and are
reachable only **after** the first per-hash mutation **of the purge transaction
in flight**, which may have been written by a **prior** invocation whose pending
record this run adopted rather than by a write this invocation performed
(§9.7.2). The catalog
is closed by PIB-228 and PIB-431; parse errors remain exit 1 and are not refusal
reports.

### 10.5 Precedence (first match wins)

1. Cobra/pflag parse, arity or mutual-exclusion error → `1`. Nothing else
   runs; the text is pflag's and is outside this command's schema, exactly as
   ADR-034 D17 scopes it.
   1a. **Command-owned flag validation** → `1`. `prepare`'s `RunE` computes the
   selected mode from §5.2's table and rejects `--yes` in every mode except
   `abandon` — including the default mode, which cobra cannot express a mutex
   against — with the single fixed literal
   `prepare: --yes is only valid with --abandon-transaction`, on stderr, with
   zero writes and no report. It runs **before** step 2, so no mode's contract
   (including the accepted read-only one) ever observes a stray `--yes`
   (§5.2, PIB-243, PIB-502, PIB-503).
2. `--check` selected → the **entire accepted contract** takes over, including
   its own precedence, codes and bytes. Nothing below applies (PIB-198).
3. Canonical slug validation → `3` (`slug-unsafe`), reusing the accepted
   grammar and its no-echo rule
   (`docs/prds/PRD-artifact-validation-and-provenance.md:696-772`).
4. Workspace discovery → `3` (`workspace-not-initialized`).
5. Read-boundary platform support → `3` (`workspace-unsupported-platform`),
   reusing ADR-034 D5's fail-closed allowlist unchanged.
6. Feature directory / `status.json` population and the accepted read
   inspection needed to plan → `3`
   (`feature-not-found`, `status-malformed`, `status-unreadable`).
   **Skipped entirely when `--abandon-transaction` is selected**: that mode's
   subject is the `.tpatch/local/` lane, not the feature directory, so it must
   not be stopped by an absent, malformed or unreadable feature. This is the
   one step above the abandon branch that rev-6 left able to refuse it, and
   removing it from that mode is what makes §6.6's gate table total (PIB-509,
   PIB-510, PIB-513).
7. **`--dry-run` branch (a real branch, not a truncation).** Everything a plan
   needs and nothing that mutates runs *inside* this branch, in order, and the
   branch always returns:
   a. read the slug's journal marker; a pending marker → `3`
      (`recovery-pending`), without strict-decoding or moving it;
   b. mode/flag grammar of §5.2 (any residual non-parse violation);
   c. lifecycle-state gate (§12.2) → `3` (`state-refused`);
   d. `request.md` capture in the generating modes → `3`
      (`request-unreadable`);
   e. artifact admissibility (§6.1.1 / §6.3) → `3` unsafe/unstable, `2`
      `present-empty` in default mode, `2` `--manual` not-ready;
   f. bundle coherence (§6.1.2, §6.1.3) → `2` (`incoherent-bundle-gap`);
   g. strict archive-index decode and X11 storage observation where the plan
      reads the index → `3` with the matching `archive-*` code;
   h. `--regenerate` provider **configuration** check → `3`
      (`provider-required-for-regenerate`); no provider call is made;
   i. emit the plan with `dry_run: true` and
      `execution_preflight: "not_evaluated"`, exit `0`.
   The branch performs **no** later step: no mutating platform/filesystem gate,
   Git/local-lane gate, directory authority, recovery, provider call,
   subprocess or write (§6.4, PIB-079, PIB-080, PIB-440, PIB-461).
8. **Directory-authority platform/filesystem support** (non-dry mutating modes
   only) → `3` (`prepare-unsupported-platform`,
   `lock-filesystem-unsupported`), §7.4.2. Distinct from step 5 because the
   write half's platform set is narrower than the read half's. When journal,
   preimage or staging evidence exists for the slug, this refusal — like a
   `directory-flock-unavailable` at step 9 — additionally names the
   repo-relative lane and the manual removal procedure (§6.6, §10.4.1).
9. One directory-flock acquisition → `3` (`transaction-in-progress`) on
   contention or `directory-flock-unavailable` on every other lock error,
   including a failure to open the workspace `*os.Root` or `root.Open(".")`
   before the lock is attempted at all (§7.4.1). In
   `abandon` mode the contention refusal offers wait-and-retry only, while
   `directory-flock-unavailable` offers the manual procedure (§6.6's gate
   table, PIB-512, PIB-536).
10. **`--abandon-transaction` → its own contract (§6.6); nothing below
    applies.** The branch is here, immediately after the lock and **before**
    the local-lane Git gate, the journal recovery and the pending-hash gate, so
    the mode never depends
    on a working `git` to move bytes already in the lane, never runs the
    recovery that would consume the evidence it exists to move, and never hits
    the exit-6 refusal it exists to escape. Steps 1, 3, 4, 5, 8 and 9 are the
    only steps that can precede it — step 1a's `--yes` validation never fires
    here because `--yes` is legal in this mode, step 2's `--check` handoff and
    step 7's `--dry-run` branch are mutually exclusive with it, and step 6 is
    skipped — and §6.6's gate table is total over exactly that set
    (PIB-452, PIB-492…PIB-494, PIB-510, PIB-511).
11. Local-lane Git contract (every mode except `abandon`) → `3`
    (`local-lane-not-ignored` or `local-lane-unverifiable`), or skipped with
    advisory `workspace-not-git` (§7.13). It still runs before the first
    local-lane or canonical byte; holding the lock across it writes nothing.
12. Pending-transaction recovery → `6` on divergent evidence, on any J1–J10
    bind failure and on a refused undo CAS; **on success it is terminal**:
    exit `0`, `outcome: "recovered"`, advisory `recovered-prior-transaction`,
    the `recovery` object and the sanitized retry. Nothing below runs in that
    invocation (§7.11, PIB-483, PIB-484).
13. Pending archive-hash state (§9.7.2) → `3` (`recovery-pending`), zero
    writes, naming the sanitized `feature intent-archive purge <slug> --all
    --yes` (or `--blob <h> --yes` where one hash covers the whole pending set)
    with `retry_cwd: "workspace-root"`. Any removal-pending reference makes the
    purge transaction the **global owner** of its hash, and ownership outranks
    every other classification of that hash, so this step fires before step 22
    can classify the same hash as dangling, residue, mixed or corrupt (§9.3).
    This step **never recovers**: the
    per-hash machine has exactly one owner and it is
    `feature intent-archive purge --yes` (§7.8 step 5, §7.11, PIB-525,
    PIB-526, PIB-539). Nothing below runs.
14. `request.md` capture (generating modes only) → `3`
    (`request-unreadable`).
15. Lifecycle-state gate (§12.2) → `3` (`state-refused`).
16. Artifact admissibility (§6.1.1 / §6.3 tables) → `3` for unsafe/unstable,
    `2` for `present-empty` in default mode, `2` for `--manual` not-ready.
17. Bundle coherence (§6.1.2, §6.1.3) → `2` (`incoherent-bundle-gap`).
18. `--regenerate` provider authority (§11.3) → `3`
    (`provider-required-for-regenerate`).
19. Generation, staging, staged-output validation → `2` on validation failure,
    `5` on an unrecoverable generation failure after staging began.
20. Set-level revalidation mismatch → `5`.
21. Redaction scan of the bytes about to be archived → `3`
    (`archive-content-refused-sensitive`), §9.6.1.
22. Archive/index/storage integrity (§9.2, §9.3.1 X1–X11), shared-selection
    refusal and purge preimage mismatch → `3`, all of it before the purge's
    first mutation. **X11's observation here is over the whole index, never
    over the selection** (§9.3.1), so an inconsistency on a hash the requested
    selector does not touch refuses this step zero-write and names that hash's
    own repair; no partial cleanup of unrelated items proceeds around it
    (PIB-542). **Admission is per repair class and requires full coverage**: a
    confirmed selection that covers every instance of the one class present is
    admitted and repairs them all in that invocation, while an uncovered
    same-class instance or any second class refuses zero-write and names each
    class's own route (PIB-548, PIB-549). A tombstoned reference whose named blob is present refuses
    here as `archive-index-storage-inconsistent`, and which repair it names is
    decided from the hash's **global** liveness: with the hash unreferenced it
    names `--orphans --yes`, and with the hash still live it names the confirmed
    `--blob <h> --yes`; each is the one invocation admitted past that
    observation, admitted only for that observation, and `--orphans` never
    removes a live blob (§9.3.1, PIB-521,
    PIB-522, PIB-531, PIB-532, PIB-542). A **retained** reference whose blob is
    present but non-regular or hash-wrong refuses here as
    `archive-blob-corrupt`, naming §9.3.1's type-total removal-then-confirmed-purge
    or restore route, and no selector is admitted past it (PIB-543, PIB-547).
    For the
    `feature intent-archive
    purge` command this ladder
    is entered only after its own journal-marker refusal (§9.7): a pending
    prepare journal is `3` `recovery-pending` before any archive work, and that
    command never recovers, decodes or moves the journal. Its `--yes` form
    additionally runs the §9.7.2 pending-hash recovery — on **every** selector,
    `--orphans` included — before this ladder, and returns terminally at `0`
    `recovered` if anything was pending; its preview form runs no recovery, takes
    no lock, and returns `0` `recovery-required` instead (§9.7, PIB-515,
    PIB-527, PIB-528, PIB-529). **That recovery is the sole exception to this
    step's pre-mutation position**: it completes a transaction that already owns
    its hash, so it runs and finalizes even while an unrelated inconsistency
    would refuse this step, and the operator's rerun — an ordinary new-selector
    invocation — is then fully governed by it (§9.3.1, §9.7.2, PIB-546). That command also
    executes **zero** Git processes in both forms, so step 11's lane gate and
    every G1–G4
    count are inapplicable to it (§7.13, PIB-514). A purge failure
    **after** the first per-hash mutation of the transaction in flight — which
    may have been written by a prior invocation — is
    instead `5` (`archive-purge-partial`), whose `resume` field says how many
    runs finish it, or `6`
    (`archive-purge-evidence-divergent`) when the evidence under a **pending**
    reference is divergent — and
    that exit 6 names §9.7.2's archive procedure, never the abandon mode
    (§9.7.2, §10.4.1, PIB-506, PIB-508, PIB-526).
23. Per-entry CAS, publication and rollback → `5` if rolled back, `6` if an
    undo CAS refused or the rollback failed.
24. Final verification (§7.8 step 13) → `6` on divergence.
25. Otherwise → `0`.

The order is load-bearing in fourteen places, and each has a row: `--yes` is
validated before the `--check` handoff, so one literal covers every illegal
spelling (PIB-502); the slug is
validated before any path is composed (PIB-176); dry-run branches before every
mutating gate, Git process and flock, and evaluates every non-mutating plan
gate inside that branch (PIB-440, PIB-461); the lock is acquired before
recovery, so recovery never races a live sibling (PIB-364); **abandon branches
after the lock and before both the Git gate and both the journal recovery and
the pending-hash gate, so its
escape is reachable for corrupt, divergent and recoverable evidence alike, and
survives a missing or unusable `git`** (PIB-452, PIB-492…PIB-494); **step 6 is
skipped in `abandon` mode, so an absent, malformed or unreadable feature cannot
stop the escape and §6.6's pre-abandon gate table is total** (PIB-509, PIB-510,
PIB-511, PIB-513); the Git gate
still runs before the first local-lane or canonical byte in every other mode
(PIB-106, PIB-107); **journal recovery returns instead of continuing, so no
exit-2 or
exit-3 gate is ever evaluated after a recovery write** (PIB-483, PIB-484,
PIB-487); **step 13 refuses instead of recovering, so the pending-purge
transaction has exactly one owning command** (PIB-525, PIB-526); recovery runs
before the lifecycle gate so a pending transaction is
never left behind by a refusal that happens to come first (PIB-177); the
redaction scan runs before
the first blob write, so a refusal leaves no sensitive bytes on disk (PIB-262);
the provider-authority gate runs before any generation, so a `--regenerate`
without a provider costs nothing (PIB-365); the whole purge preflight runs
before the purge's first mutation, so exit 3 keeps its zero-write meaning
(PIB-465, PIB-469); and **`purge --yes`'s pending-hash recovery runs before that
preflight's global X11 scan — the one exception to §9.3.1's pre-mutation
validation rule — so an unrelated inconsistency cannot brick a transaction whose
hash is already owned and half removed** (PIB-546).

### 10.6 Human output

Deterministic layout, fixed section order, no color, no wall-clock, no
duration, no absolute path, no symlink target, no artifact content:

```text
Feature: fix-model-id-translation   (state: defined)
Mode:    regenerate --allow-heuristic

  analysis.md              regenerated   (provider)   archived a91e…
  spec.md                  regenerated   (heuristic)  archived 77bd…
  exploration.md           regenerated   (heuristic)
  artifacts/analysis.json  regenerated   (provider)   archived c40a…

Archive: .tpatch/features/fix-model-id-translation/artifacts/intent-archive/blobs
  Restore a prior file with: cp <blobs-dir>/<hash>.blob <path>
  Remove archived bytes with: tpatch feature intent-archive purge fix-model-id-translation --blob <hash> --yes

Advisory: --allow-heuristic was passed, so hand-authored content was replaced
          by heuristic output where the provider failed.
Advisory: The provider call failed for spec.md and the heuristic generator was
          used instead. Re-run after fixing the provider to replace it.

Structural presence only. This report does not certify semantic quality.
```

The disclaimer is the **frozen** accepted string
(`docs/prds/PRD-artifact-validation-and-provenance.md:1796-1798`) and is
asserted byte-for-byte in both surfaces (PIB-178).

`--quiet` prints exactly one line per invocation, ending in the outcome token:

```text
prepare fix-model-id-translation: regenerate published (4 artifacts, 3 archived)
prepare fix-model-id-translation: abandon abandoned (3 control files moved)
prepare fix-model-id-translation: regenerate recovered (2 entries restored)
prepare fix-model-id-translation: generate refused incoherent-bundle-gap
feature intent-archive purge fix-model-id-translation: recovery-required (1 pending hash)
```

A terminal recovery renders in the full human output as its own short block,
which names what it restored, states that the requested operation was not
performed, and gives the sanitized retry (§7.11, §9.7.2):

```text
Feature: fix-model-id-translation   (state: defined)
Mode:    regenerate

Recovered an interrupted prepare transaction. Nothing else was done.
  restored  spec.md
  restored  status.json

Run this again from the same workspace root:
  tpatch prepare fix-model-id-translation --regenerate

Structural presence only. This report does not certify semantic quality.
```

The line is a fixed grammar — `prepare <slug>: <mode> <outcome>[ <detail>]` for
the verb's own modes, and `feature intent-archive purge <slug>: <outcome>[
<detail>]` for the archive command, which has no mode — so a harness can parse
it without knowing the mode (PIB-019, PIB-367).

### 10.7 Refusal remediation is self-contained

Every refusal names only shipped commands, shipped flags and repo-relative
paths that exist. It must not cite a PRD path, an ADR path, an issue URL or any
`docs/` file — a shipped binary's diagnostic cannot depend on a document the
user does not have. This is the accepted contract's rule
(`docs/prds/PRD-artifact-validation-and-provenance.md:374-381`), applied to
every refusal code in §10.4 (PIB-179, mechanically guarded by PIB-180).

**Three external shell forms are permitted, and no others.** A remediation may
name (1) the `cp` restore form of §9.5, (2) the lane `rm -rf` of §6.6's
environmental block and its `abandoned-*` residue line, and (3) the exact-path
`rm -rf -- <managed blobs/<hash>.blob path>`, which §9.7.2's archive-divergence
procedure and §9.3.1's corrupt-object repair share verbatim — one shape,
two observations, so the permitted set does not grow. Each is an
operator-executed
last resort for a thing tpatch deliberately refuses to do, each takes only
repo-relative paths or an explicit `<a path outside .tpatch/>` placeholder, and
each is printed with what it costs. Any other external command in any shipped
string is a guard failure (PIB-180, PIB-506, PIB-543).

**Form (3) is destructive, type-total and deliberately unaccompanied by a copy
command.** rev-10 permitted a `cp`/`rm` *pair* here and rev-11 withdraws the
`cp` half: the observations this form serves include a symlink, a directory, a
FIFO and a device node at the managed path, and a `cp` that is correct for a
regular file is wrong or fatal for the rest (§9.3.1). The permitted form is
therefore `rm -rf --` against **one** validated repo-relative managed blob path,
printed beneath an explicit warning that it deletes whatever object is there —
a directory and its contents included — with no undo. It carries the `--`
terminator so a path cannot be read as an option, and it never contains a
wildcard, a directory prefix or a second path. Where an operator wants to keep
the object, the message tells them to stop and use tooling appropriate to its
kind rather than printing one command that is right for one kind out of five.
PIB-547 asserts the form, the warning, the absence of any copy command and the
absence of any wildcard, over all five object kinds; PIB-180 keeps the permitted
set closed at three.

## 11. Path A generation

### 11.1 Pure, staged generators — the extraction

The three phase functions cannot be called (§1.3). ADR-035 D12 governs the
extraction; the implementation splits their **generation** half from their
**publication** half:

| New (pure) | Extracted from | Returns |
|---|---|---|
| `GenerateAnalysis(ctx, in AnalysisInput) (AnalysisResult, GenNote, error)` | `internal/workflow/workflow.go:35-88` (everything above the first `WriteArtifact`) | the struct plus a non-sensitive generator note; any response bytes are in-memory only and discarded after validation/render |
| `RenderAnalysisMD(result, slug) string` | already pure (`internal/workflow/workflow.go:351-401`) | markdown bytes |
| `GenerateSpec(ctx, in DefineInput) (string, GenNote, error)` | `internal/workflow/workflow.go:111-150` | markdown bytes |
| `GenerateExploration(ctx, in ExploreInput) (string, GenNote, error)` | `internal/workflow/workflow.go:159-195` | markdown bytes |

Hard constraints on the extracted functions, each source-scanned:

1. They take **no `*store.Store`** and hold no writer (PIB-184).
2. They perform **no filesystem write** of any kind (PIB-185).
3. Their inputs are plain values: request text, prior-artifact text, file tree,
   guidance text, provider handle and config.
4. `RunAnalysis`, `RunDefine` and `RunExplore` are **refactored to call them**
   and keep their current observable behavior byte-for-byte, so `analyze`,
   `define`, `explore` and `cycle` are unchanged (PIB-186, proved against
   pre-change goldens per §17.2).

`captureFileTree` and `readGuidanceFiles`
(`internal/workflow/workflow.go:249-253,290-301`) are already pure reads and
are reused unchanged.

### 11.2 Context flow and dependency order

Generation order is fixed: **analysis → spec → exploration**, matching the
shipped data dependencies (`internal/workflow/workflow.go:113-129,161-172`).

Each generator's context is assembled from the **effective** bundle — that is,
staged output from this run where it exists, and the **existing canonical
bytes** where an artifact was preserved:

| Generator | Consumes |
|---|---|
| analysis | `request.md` |
| spec | `request.md`, effective `analysis.md`, effective `artifacts/analysis.json` (when present) |
| exploration | `request.md`, effective `analysis.md`, effective `spec.md`, file tree |

Two rules make "effective" safe:

1. **Existing artifacts are read through the accepted inspector's bounded,
   rooted capture** — same cap, same refusals, same instability handling
   (ADR-034 D1, D8, D9). A preserved artifact that cannot be safely read has
   already refused the invocation at §6.1.1, so no generator ever receives
   partially-read or unsafely-resolved bytes (PIB-187).
2. **Consuming a preserved artifact claims nothing about it.** The generated
   `spec.md` does not record that its analysis was hand-authored, and the
   report does not either (PIB-142). Today's `RunDefine` already reads whatever
   `analysis.md` is on disk without asking where it came from
   (`internal/workflow/workflow.go:118-121`); this preserves that behavior
   rather than inventing a claim.

### 11.3 Provider vs heuristic — the authority policy

Default mode preserves the shipped behavior. `--regenerate` does not, and the
difference is the point.

#### 11.3.1 Default mode (`generate`): fallback, disclosed

| Situation | Behavior | Observable |
|---|---|---|
| No provider configured | heuristic generators run (`internal/workflow/workflow.go:82-84,144-146,189-191`) | `generator: "heuristic"`, advisory `provider-not-configured`, exit 0 |
| Provider configured, call fails or validation fails after retries | heuristic generator for that artifact | `generator: "heuristic"`, advisory `provider-fallback-heuristic`, exit 0 |
| A deadline expired | heuristic generator for that artifact | `generator: "heuristic"`, advisory `provider-deadline-heuristic` (plus `provider-deadline-cascade` when one total expiry caused two or more), exit 0 |
| Provider succeeds | provider output | `generator: "provider"`, no advisory |

This is safe **because the default mode only ever creates absent files**
(§6.1.3). A heuristic `exploration.md` written where none existed replaces
nothing; the operator can re-run after fixing the provider, and the advisory
says so. The provider is loaded with the **non-probing** loader `cycle` uses
(`internal/cli/phase2.go:55`), not the probing loader the single-phase commands
use (`internal/cli/cobra.go:609-612`), so a missing provider is a fallback
condition rather than a command failure — which is what makes `prepare` usable
offline, per locked-in decision 8 of `CLAUDE.md`.

#### 11.3.2 `--regenerate`: a configured, successful provider is required

rev-0 applied the same fallback to `--regenerate`. That meant a single expired
context could replace a human's three hand-authored documents with heuristic
boilerplate, report exit 0, and archive the originals into a directory the
operator did not know existed. The archive makes that *recoverable*; it does
not make it *acceptable*.

| Situation | Behavior |
|---|---|
| No provider configured | **refuse before any mutation**: exit 3, `provider-required-for-regenerate`; zero blobs, zero canonical writes; remediation names `tpatch provider set`, `tpatch provider check` and `--allow-heuristic` |
| Provider configured but unreachable, or every retry fails | **refuse before publication**: exit 5, `regenerate-generation-failed`; staging retained; zero blobs written (the redaction scan and blob writes come *after* staging, §7.8) |
| Any deadline expires during generation | same refusal, with the expired deadline named |
| `--allow-heuristic` passed and any of the above occurs | proceed with heuristic output; advisory `regenerate-heuristic-allowed` **plus** the specific fallback advisory; exit 0 |
| Provider succeeds for every artifact | publish; exit 0 |

**One expiry cannot silently cascade.** In `--regenerate` without
`--allow-heuristic`, the first failure refuses the whole invocation, so a
cascade is impossible by construction. In default mode and under
`--allow-heuristic`, a total-deadline expiry that causes two or more fallbacks
emits `provider-deadline-cascade`, which states that **one** expiry — not three
independent provider failures — produced them, so the report cannot be
misread as "the provider is broken" (PIB-368 … PIB-373).

**`--allow-heuristic` is the complete opt-in, and it is spelled out.** Its help
text reads: *"Permit `--regenerate` to replace the bundle with heuristic
output when the provider is missing or fails. Without this flag, regeneration
refuses rather than downgrading hand-authored content."* It is legal only with
`--regenerate` (a no-op advisory in default mode, a parse error elsewhere,
§5.2), it is never implied, and there is no config key that turns it on
globally — an opt-in that can be set once in a file and forgotten is not an
opt-in (PIB-374, PIB-375).

- Fallback, where it is permitted, still yields a **complete** new set, so
  WP-005 Agreed item 7's "complete prior set or complete new set"
  (`docs/whitepapers/WP-005-spec-driven-workflows.md:80-81`) holds: a provider
  failure never produces a half-bundle.
- The heuristic sidecar keeps `heuristic_mode: true`
  (`internal/workflow/workflow.go:25,208`), byte-compatible with what
  `analyze` writes today. That field is pre-existing persisted data, not a new
  provenance representation (PIB-146).
- **There is no `--require-provider` in v1.** It would be redundant:
  `--regenerate` requires one already, and default mode's fallback writes only
  files that did not exist. Q3 records the residual question.

### 11.4 Retry without transcript retention

Retry uses the shipped `GenerateWithRetry` corrective-prompt behavior and
`--no-retry` mapping, but the prepare construction always leaves
`RetryOptions.Store` nil. It supplies no response sink. A pure generator may
hold one provider response in memory only long enough to validate and render its
intended canonical output; it then discards it. It never stages, archives,
journals, reports or retains provider response text, prompt text, corrective
prompt text or source bodies.

If attempt diagnostics are needed for control flow, their only persistent shape
is bounded redacted/hashed metadata: attempt ordinal, closed validation-reason
class and optional SHA-256/size. No metadata permits recovery of text. Failure
retention names only allowed staged canonical outputs/control residue, never a
transcript. This is stricter than the shipped phase path and is required by
ADR-027 D2/D3 (PIB-188…PIB-190, PIB-376, PIB-419, PIB-420).

### 11.5 Timeout budget — per-phase and total

Two deadlines, both flags, both enforced:

| Deadline | Flag | Default | Applies to |
|---|---|---|---|
| **Total** | `--timeout` | `180s` | all generation in the invocation, measured from the first generator call |
| **Per-phase** | `--timeout-phase` | `90s` | each individual generator call, clamped to the remaining total |

Each generator runs under `context.WithTimeout(parent, min(perPhase,
remainingTotal))`. The single-phase commands each use `60s`
(`internal/cli/cobra.go:629`); `prepare` may make three calls, so a single 60s
budget would fail routinely. The clamped deadlines bound **provider-generation
context only**; they do not bound the command or authority lifetime.

- **The deadlines cover generation only.** They **cannot** interrupt the
  filesystem reads of the inspection half — ADR-034 D16 withdrew every
  bounded-runtime claim, and this PRD does not resurrect one. Filesystem, Git,
  recovery, publication, fsync and release have no hard wall-clock bound
  (PIB-191, PIB-437).
- **A per-phase expiry** is a generation failure for that artifact: fallback in
  default mode (advisory `provider-deadline-heuristic`), refusal in
  `--regenerate` without `--allow-heuristic` (PIB-192, PIB-369).
- **A total expiry** ends provider use for the whole invocation: every
  remaining artifact is either heuristic (default mode / `--allow-heuristic`,
  with `provider-deadline-cascade` when two or more are affected) or the
  invocation refuses (`--regenerate`) — PIB-370, PIB-371.
- **Failure is decided before publication, never during it.** No deadline is
  consulted after staging validation succeeds. If the total deadline expires
  after staging and before publication, the command **publishes anyway**: the
  deadline governs generation, not the transaction, and aborting a validated
  publication because a clock expired would be strictly worse (PIB-193).
- The deadline values never reach a tracked artifact or a report field
  (no durations in output, §13.4) — PIB-377.

### 11.6 Staged-output validation — before any canonical write

Every staged byte passes all of these before the window can open. Failure of
V1–V5 is exit `2`; failure of V6 is exit `5`, because a staged file whose
identity moved is a transaction-integrity failure rather than an output-quality
one. In both cases: zero canonical mutations, staging retained.

| # | Check | Rationale |
|---|---|---|
| V1 | non-empty after whitespace trim | the accepted `present-empty` rule (`docs/prds/PRD-artifact-validation-and-provenance.md:1765`) — publishing an artifact the check would call empty is incoherent |
| V2 | size ≤ the accepted `MaxArtifactBytes` | publishing something the check would call `oversize` is incoherent |
| V3 | no NUL byte | these are Markdown/JSON text artifacts; a NUL indicates a truncated or binary provider response |
| V4 | valid UTF-8 | same |
| V5 | sidecar parses as a JSON **object** | the accepted `invalid-structured` rule (`docs/prds/PRD-artifact-validation-and-provenance.md:1772`); the shipped validator already enforces object-ness at generation time (`internal/workflow/workflow.go:62`) |
| V6 | the staged file's post-fsync identity equals the identity recorded for it | nothing may be published that was not durably staged |

**V1–V6 are structural only.** No check inspects headings, length,
plausibility, or whether the text is on-topic. §14.1's disclaimer is what the
report says instead (PIB-194).

**The self-consistency invariant.** After a successful `published` outcome,
running the accepted `prepare <slug> --check` in the same tree must report
`ready` and exit 0. This is the single strongest end-to-end assertion in the
matrix (PIB-195) and is what V1/V2/V5 exist to guarantee.

## 12. Lifecycle and compatibility

### 12.1 `--check` is byte-identical

ADR-035 D14 freezes it. The accepted mode keeps its grammar, its precedence, its report schema, its
exact bytes, its exit envelope and its zero-mutation contract. The only
permitted change to the shipped `prepare` command file is the addition of the
new flags and the mode dispatch **above** the `--check` path. Regressions:
PIB-198 (bytes, human), PIB-199 (bytes, JSON), PIB-200 (exit codes across all
populations), PIB-201 (zero mutation), PIB-202 (`provenance: unknown`
constant), PIB-203 (disclaimer string), PIB-204 (abort shapes), PIB-205
(`artifacts` ⇔ `abort` invariant), PIB-207 (byte-identical with a pending
journal).

### 12.2 Allowed source states

| State | Mutating `prepare` | Why |
|---|---|---|
| `requested` | **allowed** | The ordinary entry point. Generates the whole bundle; ends `defined`. |
| `analyzed` | **allowed** | Completes the bundle from an existing analysis. |
| `defined` | **allowed** | Idempotent: completes a missing `exploration.md`, adopts under `--manual`, or regenerates. May be a no-op success (§6.1.4). |
| `implementing` | refused, exit 3 | A recipe already derives from the current intent (`internal/workflow/workflow.go` implement phase; `docs/feature-layout.md:85`). Rewriting intent underneath it desynchronizes the pair with nothing recording the mismatch. |
| `applied`, `active` | refused, exit 3 | A canonical patch exists (`docs/feature-layout.md:19,34`). Same argument, plus `verify`'s later `intent_files_present` check would compare against changed intent. |
| `reconciling`, `reconciling-shadow` | refused, exit 3 | A reconcile session is live; its inputs must not move under it. |
| `blocked` | refused, exit 3 | Post-implementation; resolve the blocker first. |
| `upstream_merged` | refused, exit 3 | Terminal, and asserts the opposite verdict from "prepare this feature" (`internal/store/types.go:26-31`). |
| `rejected` | refused, exit 3 | Terminal pre-implementation verdict; the shipped exit route is `tpatch reopen` (`internal/store/status.go:139-152`). |
| `unapplied` | refused, exit 3 | Delegated verbatim to the shipped `refuseIfUnappliedState` helper (`internal/cli/feature_unapply.go:464-473`), which already returns exit 3 and already names `tpatch apply` as the remedy. |

The table is **total** over the twelve shipped states
(`internal/store/types.go:9-38`); PIB-225 asserts totality mechanically against
`ValidFeatureState` (`internal/store/types.go:40-47`), so a thirteenth state
added later cannot silently default to "allowed".

**Interaction with reject/reopen.** `prepare` never rejects and never reopens.
A rejected feature refuses with a remediation naming `tpatch reopen <slug>`; a
reopened feature returns to its `PriorState`
(`internal/store/status.go:130-133`) and is then subject to this same table
(PIB-127, PIB-128).

### 12.3 Exact status metadata written

On a successful mutating publication, and only then:

| Field | Value |
|---|---|
| `state` | `defined` (`internal/store/types.go:11`) |
| `last_command` | `"prepare"` |
| `updated_at` | refreshed by the shipped writer (`internal/store/store.go:388`) |
| `notes` | one of exactly three frozen strings (below) |
| everything else | **untouched**, including `verify`, `rejection`, `rejection_history`, `depends_on`, `apply`, `reconcile`, `compatibility`, `id`, `title`, `requested_at` |

Frozen notes strings, one per publishing mode:

```text
Intent bundle prepared (prepare); generated: analysis.md, spec.md
Intent bundle adopted (prepare --manual); artifacts authored by hand
Intent bundle regenerated (prepare --regenerate); prior bytes archived
```

The generated-file list in the first form is the publication set's Markdown
entries in canonical order, so the string is deterministic for a given plan.
`--abandon-transaction` publishes nothing and therefore writes **no** note and
does not touch `status.json` at all (PIB-270).

**`Verify` is not touched.** ADR-013 makes `verify` and `amend` the only
writers of that record (`internal/store/types.go:236-251`), and `prepare` is
neither. This is asserted, not assumed (PIB-129). Note the honest consequence:
a feature that was `verified-fresh` before a `--regenerate` keeps that label
even though its intent changed. That is existing `verify` freshness semantics —
`define` has exactly the same effect today — and this PRD does not widen
`verify`'s writer set to fix it. Q6 records it.

#### 12.3.1 `FEATURES.md`: derived, best-effort, outside T1

`SaveFeatureStatus` writes `status.json` atomically and then calls
`RefreshFeaturesIndex`, **swallowing its error**, because "status.json is the
source of truth and must land even if the derived index can't be rewritten"
(`internal/store/store.go:363-377`, and the rebuild itself at
`internal/store/store.go:703-712`). Every state transition in the product
already behaves this way. rev-0 described `--manual` as writing "exactly one
file", which was false.

The contract (ADR-035 D20), stated once and applied to every mode:

| Property | Value |
|---|---|
| In the publication set? | **No.** `FEATURES.md` is derived from every feature's `status.json`, not from this feature's intent. |
| Journaled? | **No.** No preimage, no new image, no CAS. |
| Rolled back? | **No.** A rollback restores `status.json`; it does not restore the index. |
| Covered by T1? | **No.** T1 is scoped to the publication set (§7.1), and this file is explicitly named as outside it. |
| Failure behavior | Swallowed by the shipped writer; `prepare` additionally **reports** it as advisory `features-index-refresh-failed`, which today's phase commands do not (a new-surface improvement, §12.6 D10). |
| Reconvergence | Any later `SaveFeatureStatus` — from `prepare`, any phase command, `reject`, `reopen`, `record`, `land` — rebuilds it from the authoritative `status.json` files. |

**Two consequences are stated rather than hidden:**

1. **The `--manual` write set is `{status.json}` in the publication set plus a
   best-effort `FEATURES.md` rewrite.** The single-rename atomicity claim of
   ADR-035 D3 applies to the publication set, not to the process's whole write
   footprint (PIB-257, PIB-258).
2. **After a rollback or an undo, `FEATURES.md` may briefly name a state that
   `status.json` no longer claims.** To bound that, every rollback and every
   recovery that restores `status.json` re-runs the same best-effort refresh as
   its last act, so the derived index reconverges within the same invocation
   whenever the filesystem permits it (PIB-259, PIB-260). If it does not, the
   advisory fires and the next transition fixes it. No row anywhere claims
   `FEATURES.md` is transactional (PIB-261).

**Why not make it transactional.** Bringing `FEATURES.md` into the publication
set would make every `prepare` CAS a file that eleven other commands rewrite
without a lock, turning ordinary concurrent activity into exit-5 aborts on a
*derived* file. The shipped design deliberately treats it as a cache; this PRD
matches that and says so.

#### 12.3.2 `notes` is a last-transition hint, not provenance

`MarkFeatureState` overwrites `Notes` on every transition
(`internal/store/store.go:380-393`), so the field describes **the most recent
transition only**. It is not a log, not an append-only record, and not evidence
about who authored any artifact. Three rules follow, each asserted:

1. No report field, no doc sentence and no skill text may present `notes` as
   evidence of Path A vs Path B authorship — the correction the accepted
   contract already requires for `docs/agent-as-provider.md`
   (`docs/prds/PRD-artifact-validation-and-provenance.md:3372-3435`) and which
   §14.1 carries out (PIB-142, PIB-220).
2. The archive does not upgrade `notes` into provenance: a
   `prepare --regenerate` note records that *this invocation* regenerated the
   bundle, not where the replaced bytes came from (§9.8, PIB-378).
3. The frozen strings above are **state**, not history: they are overwritten by
   the next transition, and the wording is chosen so that reading a stale one
   cannot mislead — each names the command that wrote it, and none claims a
   property of the current files (PIB-379).

### 12.4 The mutating `--manual` gates are unchanged

`analyze|define|explore|implement --manual` keep their current loose,
presence-only behavior (`internal/store/manual.go:56-81`). The accepted PRD
made that decision deliberately
(`docs/prds/PRD-artifact-validation-and-provenance.md:2986-3015`) and this PRD
does not reopen it. In particular:

- `tpatch define <slug> --manual` on a zero-byte `spec.md` still advances to
  `defined` and still exits 0 (PIB-130).
- `tpatch prepare <slug> --manual` on the same tree refuses with exit 2
  (PIB-131).
- The differential is intentional and is a composite acceptance row, not a
  footnote: the bundle verb is strict, the per-artifact verbs stay loose
  (PIB-132).

### 12.5 `doctor` gains one evidence-only check

D9 is warning-only and makes zero writes. It reports **persistent evidence
only**: a present journal, allowed staging or `abandoned-*` control residue, an
orphan blob — including a blob left unreferenced beside a tombstone, which it
names with the `--orphans --yes` repair (§9.3.1 subcase A) — a **mixed
tombstone/live-reference** hash, which it names with the confirmed
`--blob <hash> --yes` repair and never with `--orphans` (§9.3.1 subcase B) — a
**corrupt** blob under a retained reference, which it names with §9.3.1's
type-total removal-then-confirmed-purge or restore route — an **owned** hash, any
reference to which is removal-pending, which it names with that transaction's
`purge … --yes` route (§9.3) — a
corrupt archive index, or an X11 dangling reference; for a
dangling reference it names the same single literal repair command as `list`
and the refusal report (§9.7.3). Its observation is **global**: it scans the
whole index, and where several of these coexist it reports **every** one of
them with its own repair rather than the first or the worst, exactly as `list`
does — and where several instances of **one** class coexist it reports them as
one class with many instances, carrying the single invocation that clears all of
them rather than implying one run per instance (§9.7, PIB-541, PIB-548). It must not
diagnose a lost journal from ordinary canonical partial bytes (§7.11.1), and it
does not claim a persistent or inert lock locator exists, because none does
(§7.4.1).

**D9 does not probe the lock, and no diagnostic command does.** rev-4 permitted
a non-creating nonblocking flock attempt on the workspace root. That is removed,
for two independent reasons:

1. **A diagnostic must not perturb a mutator.** The probe takes a real
   `LOCK_EX|LOCK_NB` on the real authority inode. Between its acquisition and
   its release, a genuine mutating `prepare` that starts contends and refuses
   exit 3 `transaction-in-progress` — a refusal manufactured by the diagnosis.
   "It releases immediately" is a narrower window, not a different class:
   `tpatch doctor` is exactly the command an operator or CI runs *alongside*
   real work, and a health check that can fail the thing it is checking is a
   defect regardless of window size.
2. **Concurrent doctors diagnose each other.** Two `doctor` runs on one
   workspace would each observe the other's probe and each report
   `workspace mutation authority held`, with no holder identity available to
   either — a report that is true of the diagnostics and false of the workspace.

So D9 opens nothing, locks nothing and reports only what is durably on disk
(PIB-470). The information the probe would have produced is not recoverable by
any other means either, and this PRD says so instead of substituting a weaker
probe: **no diagnostic command can identify a lock holder or prove that no
holder exists.** `transaction-in-progress` is the only place the authority is
ever reported, it is emitted by the mutator that actually contended, and its
text says exactly three things — the workspace mutation authority is held, the
holder's identity is unknowable, and the safe action is to wait and retry. No
shipped string anywhere claims more (PIB-471).

Existing D1–D8 doctor goldens are preserved byte-for-byte. A clean lane/archive
and an ordinary partial bundle get no invented journal diagnosis
(PIB-134…PIB-136, PIB-380…PIB-387, PIB-407, PIB-428, PIB-445, PIB-470,
PIB-471).

### 12.6 Enumerated behavior deltas — the complete list

Nothing outside this table changes.

| # | Delta | Surface |
|---|---|---|
| D1 | `tpatch prepare <slug>` stops refusing with exit 4 and performs a Path A run | `prepare` |
| D2 | `--manual`, `--regenerate`, `--abandon-transaction`, `--allow-heuristic`, `--dry-run`, `--timeout`, `--timeout-phase`, `--no-retry` and `--yes` become registered flags; supplying an illegal combination changes the exit-1 message text from `unknown flag` to cobra's mutual-exclusion text — except `--yes` outside `--abandon-transaction`, whose exit-1 text is this command's own fixed literal (§5.2, §10.5 step 1a) | `prepare` |
| D3 | `prepare` persists no raw retry attempt/transcript, prompt or source-body sink; retry metadata is redacted/hashed only, and intended canonical staged output remains publication output rather than history | `prepare` (new surface only) |
| D4 | A new tracked directory `artifacts/intent-archive/` can appear under a feature, and `land` sweeps it into the operator's commit like any other artifact file | `.tpatch/features/<slug>/` |
| D5 | `prepare` prints progress to stderr, not stdout | `prepare` (new surface only) |
| D6 | `doctor` gains check `D9` (§12.5) — evidence-only, zero writes, and it never opens or flocks the workspace root | `doctor` |
| D7 | `RunAnalysis`/`RunDefine`/`RunExplore` are refactored to call the extracted pure generators; their observable behavior is unchanged and golden-pinned | `analyze`, `define`, `explore`, `cycle` |
| D8 | **Mutating** `prepare` supports Linux/Darwin only after the locked-root-inode denylist check and a real flock succeed; Windows, BSD and all other targets refuse. This is narrower than `--check`'s accepted `unix \|\| windows` read allowlist and does not classify nested mounts. | `prepare` (mutating modes only) |
| D9 | **Mutating** `prepare` refuses with `local-lane-unverifiable` unless G1 can establish either a Git worktree or a non-worktree; a Git-established non-worktree proceeds with an advisory. `--abandon-transaction` is exempt: it runs zero Git processes and cannot emit either lane refusal (§6.6, §7.13). `feature intent-archive list` and `feature intent-archive purge` are likewise exempt and run zero Git processes, because neither writes into the gitignored lane (§7.13, PIB-514). `define --manual` remains unchanged. | `prepare` (mutating modes only) |
| D10 | `prepare` **reports** a failed `FEATURES.md` refresh as an advisory; the shipped phase commands swallow it silently. `status.json` remains authoritative in both (§12.3.1) | `prepare` (new surface only) |
| D11 | A new command group `tpatch feature intent-archive {list,purge}` appears (§9.7) | `feature` |
| D12 | A prepare-owned held-root directory authority retains `*os.Root` and `*os.File`, uses the root-inode denylist plus real flock on Linux/Darwin, and is not a `rescap` extraction; `feature resource *` remains byte-identical. | `prepare` (new mutating surface only) |
| D13 | A mutating `prepare` that finds a pending **journal** recovers and stops: exit 0, `outcome: "recovered"`, and the operator re-runs the reported sanitized command. A mutating `prepare` that finds a pending **archive purge** instead refuses exit 3 `recovery-pending` with zero writes and names `feature intent-archive purge <slug> … --yes`, which is the one command that finalizes it — and which likewise recovers and stops rather than continuing into the selector. No shipped command performs a recovery and the requested operation in one invocation, and no command other than `intent-archive purge --yes` finalizes a pending archive hash. | `prepare`, `feature intent-archive purge` (new surface only) |

**Non-invalidation obligations**, each with a row:

- `next`'s routing is byte-identical for every state, including the
  `exploration.md`-presence branch (`internal/cli/phase2.go:437-446`) — PIB-208
  against pre-change goldens.
- `cycle` is byte-identical end to end — PIB-209.
- `analyze`, `define`, `explore`, `implement`, with and without `--manual`, are
  byte-identical — PIB-210, PIB-211.
- `status`, `verify`, `record`, `land`, `reconcile`, `doctor` D1…D8 are
  byte-identical for a feature that never runs `prepare` — PIB-212, PIB-136.
- Every `tpatch feature resource` subcommand is byte-identical after this
  cluster's changes — `rescap`'s file lock is not extracted, moved or altered
  (§7.4.4) — including its contention refusal text and its unsupported-platform
  refusal — PIB-286, PIB-287.
- No command gains a `prepare` precondition, and nothing calls `prepare`;
  asserted in both directions by a reverse call-graph guard — PIB-213.
- Every existing caller of the Git ignore/tracked helpers in
  `internal/gitutil/ignore.go`, `internal/workflow/session_ignore.go`,
  `internal/rescap/scratch.go` and `internal/rescap/gitgate.go` keeps its
  current environment, call shape, exit-code interpretation, output and
  goldens through its explicit compatibility wrapper; the central-executor
  refactor moves the spawn seam, not the semantics — PIB-472, PIB-473.
- `prepare` uses only the conditional read-only G1–G4 allowlist of §7.13:
  G1–G3 for a worktree mutating run, G4 only for regenerate, G1 alone for an
  established non-worktree, and **none** for `--check`, dry-run,
  `--abandon-transaction`, `intent-archive list` or `intent-archive purge`; the
  Git index, `.git/**` and every worktree are untouched — PIB-106,
  source-scanned by PIB-107, with the purge zero asserted by PIB-514.

## 13. Security, privacy and determinism

### 13.1 Reads reuse the accepted boundary, unchanged

Every canonical read — inspection, generation context, revalidation — goes
through the accepted rooted capture and its refusals: one held `*os.Root`,
`fs.ValidPath` root-relative names, observed-symlink refusal, non-regular
refusal, the bounded `Max+1` read, the identity checks and the fail-closed
`unix || windows` platform allowlist (ADR-034 D1–D9, D15). No second path model
is introduced, and `filepath.Join(repoRoot, …)`, `os.Stat`, `os.Lstat`,
`os.Open`, `os.ReadFile` remain forbidden in the read path (PIB-181,
source-scanned).

### 13.2 Writes: rooted primitives, final-leaf refusal, and disclosed in-root redirects

Writes are a **new** surface and ADR-035 D2 governs them:

1. Every write target is one of a **closed, compile-time list** of
   root-relative names derived from the validated slug: the four artifacts,
   `status.json`, `artifacts/intent-archive/index.json`,
   `artifacts/intent-archive/blobs/<hash>.blob`, and the `.tpatch/local/` lane.
   No write path is ever composed from provider output, from a report field, or
   from any file's content (PIB-182).
2. **Every write is handle-relative** — `(*os.Root).OpenFile`,
   `(*os.Root).Rename`, `(*os.Root).Remove`, `(*os.Root).Mkdir` — so no
   ancestor redirect can escape the held root, and no path-based
   `os.Rename`/`os.CreateTemp`/`os.WriteFile` follows a rooted check (§7.7,
   PIB-308, PIB-310). This is total over the command's write footprint: the
   tracked lane, the gitignored `.tpatch/local/` lane and the derived-index
   refresh all use it, and neither `gitutil.DurableWriteFile` nor
   `writeFileAtomic*` is called from any of them (§7.5, §7.7.1, §7.7.3,
   PIB-454, PIB-456). A relative ancestor symlink that remains in-root may
   still redirect the name; CAS detects a changed resolved identity but not an
   equal-identity alias (§7.7.2, PIB-406).
3. **Final publication targets are never followed.** Immediately before each rename,
   the target's `Lstat` through the held root must show either non-existence or
   a **regular file**. A symlink, junction, directory, FIFO, socket or device
   at that final target aborts the transaction (exit 5) — it is never replaced
   or written through (PIB-183, PIB-196, PIB-197). This does not make an
   ancestor redirect impossible.
4. Temp files are created **inside the destination directory**, root-relative,
   with `O_CREATE|O_EXCL`, so the rename is same-filesystem — the shipped
   rationale (`internal/store/store.go:884-886`) reimplemented on the rooted
   primitive rather than reused path-based (§7.7.3).
5. Modes: `0644` for new tracked files, preserved permissions on replacement
   (`internal/store/store.go:871-876`), `0700`/`0600` in `.tpatch/local/`.

### 13.3 Bounded provider outputs

Provider responses are bounded twice: by the request's `MaxTokens`
(`internal/workflow/workflow.go:58`, `:134`, `:179`) and, decisively, by V2's
byte cap on the staged file (§11.6). A response that exceeds the cap fails
validation and is never published (PIB-157).

### 13.4 No raw secret or provider-response leakage

- Config stores an env-var **name**, never a secret
  (`internal/store/store.go:72`), and `prepare` never reads, echoes or
  persists the value.
- Prepare creates no raw provider-response, prompt, corrective-prompt or
  transcript persistence lane (§11.4). Attempt bytes are in memory until
  validation/render, then discarded; the one exception is the intended
  canonical staged output, which may equal the provider result and is governed
  as publication output rather than attempt history. This is stricter than
  `analyze` today, which writes raw response text to `artifacts/`
  (`internal/workflow/retry.go:105-109`).
- No report field carries artifact content, a symlink target, an absolute path,
  a hostname, a PID, a duration or a wall-clock timestamp. The forbidden-field
  guard scopes to **key names and human labels**, not raw substrings, so a
  legitimate value like `archived_blob` cannot be made unspellable — the
  scoping lesson from the accepted contract
  (`docs/prds/PRD-artifact-validation-and-provenance.md:2554-2556,3626`) — PIB-158,
  PIB-159. The same scoping rule governs the provenance guard (§10.2): it walks
  declared schema keys and the enumerated persistence sinks of §13.5, never
  `.tpatch/` bytes, because canonical and provider prose may contain any word
  (PIB-144, PIB-477).
- **Blob content is the artifact's own bytes, and that is a retention decision,
  not a non-event.** §9.6.1 makes the ADR-027 D3 redaction scan a precondition
  for writing one, and §9.6.2 discloses the durable retention it creates. The
  redaction refusal's own message names only artifact ids and class codes,
  never matched bytes, offsets or excerpts (PIB-263, PIB-388).

### 13.5 Tracked vs local artifacts — the complete split

| Path | Tracked? | Lifetime |
|---|---|---|
| `.tpatch/features/<slug>/{analysis,spec,exploration}.md` | tracked | canonical |
| `.tpatch/features/<slug>/artifacts/analysis.json` | tracked | canonical |
| `.tpatch/features/<slug>/artifacts/intent-archive/index.json` | tracked when `.tpatch/` is (§9.6.2) | durable until purged; tombstones survive purge |
| `.tpatch/features/<slug>/artifacts/intent-archive/blobs/*.blob` | tracked when `.tpatch/` is (§9.6.2) | durable until purged; content-immutable while present |
| `.tpatch/features/<slug>/status.json` | tracked | canonical |
| `FEATURES.md` (repo root) | tracked | derived, best-effort, outside T1 (§12.3.1) |
| `.tpatch/local/intent-prepare/<slug>/journal.json` | **gitignored** | one transaction |
| `.tpatch/local/intent-prepare/<slug>/{index,status}.preimage.json` | **gitignored** | one transaction |
| `.tpatch/local/intent-prepare/<slug>/stage-*/**` | **gitignored** | one invocation (retained on failure) |
| `.tpatch/local/intent-prepare/<slug>/abandoned-*/**` | **gitignored** | until the operator removes it (§6.6) |

The `.tpatch/local/` ignore contract is enforced before any byte is written
there, conditionally on Git state per §7.13, and a tracked file anywhere
under `.tpatch/local/` refuses the command (PIB-186, PIB-187).

### 13.6 Determinism

- No wall-clock field in any tracked artifact this command writes, and none in
  the journal either (§7.5). The only clock that moves is `status.json`'s
  pre-existing `updated_at`, written by the shipped `SaveFeatureStatus`
  (`internal/store/store.go:369-371`) — pre-existing behavior of every state
  transition, not a new field.
- Canonical JSON with fixed key order and **no Go map** in any wire format
  (ADR-033 D11, `internal/store/canonjson.go:11-17`) — PIB-165.
- The archive is a pure function of replaced content (§9.4).
- The JSON report's key order is fixed and its arrays are ordered by the
  canonical artifact order, never by map iteration — PIB-166.
- Two identical runs against identical trees with the heuristic generator
  produce byte-identical artifacts and byte-identical reports except for
  `status.json`'s `updated_at` — PIB-167.

## 14. Docs, assets and SPEC parity

### 14.1 Documents the implementation wave must update

| Document | Required change |
|---|---|
| `SPEC.md` | The `prepare` surface: five modes, the complete flag mutex table, the seven-code exit envelope with exit 4 recorded as retired and exit 5 carrying both the publication-abort and `archive-purge-partial` populations (§10.4), the publication-unit statement (including that `FEATURES.md` is outside it), the narrower mutating-platform envelope (§12.6 D8) and the `tpatch feature intent-archive` grammar. |
| `docs/feature-layout.md` | A new subsection for `artifacts/intent-archive/` — what it is, that it is tracked when `.tpatch/` is, that blob **content** is immutable but blobs are **removable**, that it is never canonical truth, the `cp` restore form, pending/tombstone/orphan semantics (§9.7), the durable-retention disclosure (§9.6.2), the accidental-secret route and the **single** dangling-reference repair of §9.7.3, and the journal-loss boundary (§7.11.1). |
| `docs/agent-as-provider.md` | A `prepare --manual` row alongside the per-phase `--manual` table (`docs/agent-as-provider.md:40-45`), stating that it adopts the **whole** bundle and is strict where the per-phase gates are loose. The existing sentence presenting `status.json.notes` as what "distinguishes Path B transitions from provider output" (`docs/agent-as-provider.md:47-54`) must additionally be corrected to a last-transition hint, **not** durable per-artifact provenance — a correction the accepted PRD already requires (`docs/prds/PRD-artifact-validation-and-provenance.md:3372-3435`) and which this PRD must not contradict. |
| `docs/path-b-operator-guide.md` | The three-`--manual`-commands flow (`docs/path-b-operator-guide.md:61-73`) gains `tpatch prepare <slug> --manual` as the one-step adoption alternative. |
| `docs/path-b-operator-guide.md` (second change) | The non-Git and unusable-Git behavior of §7.13, so a Path B operator learns the `local-lane-unverifiable` refusal from the guide rather than from the refusal. |
| `CHANGELOG.md` | The thirteen deltas of §12.6. |
| `docs/adrs/README.md` | The ADR-035 index row (created with this PRD at rev-0; update its proposed rev-11 status when implementation is authorized). |

### 14.2 Skill asset parity

All six shipped skill surfaces must name the command, and the parity guard must
be extended:

1. `requiredCommands` (`assets/assets_test.go:14-53`) gains `tpatch prepare`.
2. `requiredAnchors` (`assets/assets_test.go:62-73` is the shipped shape) gains
   three anchors: one for the preservation default, one for the archive, and one
   for the `--regenerate` provider requirement, so removing or paraphrasing any
   of them fires the guard.
3. **Hard constraint — the non-mandate assertion.** `prepare` must **not** be
   added to the skills' phase-ordering table or preflight sequence
   (`assets/assets_test.go:66-73`). WP-005 Agreed items 3 and 6
   (`docs/whitepapers/WP-005-spec-driven-workflows.md:56-59,73-76`) forbid
   making a bundle methodology mandatory; a skill that lists `prepare` as a
   required step would do exactly that in the surface agents actually read.
   PIB-216 asserts the command appears in all six files; PIB-217 asserts it
   appears in **no** phase-ordering or preflight block in any of them.
4. Skill text must not claim the command certifies quality, and must not claim
   provenance (PIB-218).
5. Skill text must not present the archive as a general history or undo
   facility, and must state that `--regenerate` requires a provider unless
   `--allow-heuristic` is passed (PIB-389, PIB-390).

## 15. Migration

No migration step, no backfill, no rewrite of any existing file.

| Existing situation | Behavior on first `prepare` |
|---|---|
| Feature with all three Markdown files, `defined` (Path A or Path B) | default mode: no-op success, zero bytes written (PIB-035) |
| Feature with `analysis.md` + `spec.md`, no `exploration.md`, `defined` | default mode: generates `exploration.md` only; `analysis.md`, `spec.md` and the sidecar untouched (PIB-030) |
| Path A feature with a sidecar | sidecar untouched unless `analysis.md` is generated this run (PIB-032) |
| Path B feature with no sidecar | no sidecar is synthesized; advisory `analysis-preserved-sidecar-untouched` (PIB-033) |
| Feature with a zero-byte `spec.md` that reached `defined` through the loose `--manual` gate | default mode refuses (exit 2); `--regenerate` archives the zero-byte file and replaces it (PIB-034, PIB-067) |
| Legacy feature whose `status.json` predates any field added since | untouched; `omitempty` round-trip preserved (PIB-137) |
| Feature with no `artifacts/` directory | created on demand with `MkdirAllAndSyncChain`; no other feature touched (PIB-138) |
| G1-established Git worktree whose `.tpatch/` is untracked | works; advisory `bundle-untracked-in-git`, and no clone-durability is claimed for the archive (PIB-139, PIB-342) |
| Workspace Git establishes as not-a-worktree | works in every mutating mode; the local-lane gate is inapplicable; advisory `workspace-not-git` (PIB-327) |
| Workspace where G1 cannot execute or classify Git | mutating modes refuse with `local-lane-unverifiable`; the per-phase `--manual` commands still work (PIB-329) |
| BSD or another unsupported mutating platform | mutating modes refuse with `prepare-unsupported-platform`; accepted `--check` behavior is unaffected (PIB-288, PIB-289, PIB-409) |
| Archive written by a **newer** `tpatch` (`schema_version` ahead) | every mode that reads the index refuses with `archive-index-version-unsupported` and says to upgrade, never that the file is broken (PIB-333) |
| A feature that never runs `prepare` | byte-identical in every command (PIB-212) |

The archive directory is created **lazily**, only by the first `--regenerate`
that actually replaces something. A repository that never regenerates never
grows one (PIB-068).

## 16. Risks

| # | Risk | Mitigation |
|---|---|---|
| R1 | An operator reads "transaction" as T0 and assumes concurrent readers are safe. | §7.1's three-way table is normative; the term "atomic" is never applied to the multi-file publication in any shipped string; PIB-155 is a mechanical over-claim guard over every shipped message and doc sentence this PRD owns. |
| R2 | `--regenerate` destroys work despite the archive, because the archive step silently failed. | Archive failure refuses the whole invocation before the window (§6.3); PIB-060…PIB-066 cover each failure shape. The redaction gate refuses *before* the first blob write, so a refusal never leaves sensitive bytes behind (§9.6.1, PIB-262). |
| R3 | The undo-only journal is mistaken for land's redo journal by a future implementer. | ADR-035 D5 states the direction and its reason; §4's table contrasts them; the journal carries no `phase` field to decide from. |
| R4 | Extraction of the generators regresses `analyze`/`define`/`explore`/`cycle`. | S2 lands pre-change goldens **before** the refactor; PIB-186, PIB-208…PIB-211 compare against them. A refactor with no golden is exactly the no-op-vs-no-op comparison the accepted PRD warns about (`docs/prds/PRD-artifact-validation-and-provenance.md:3515-3520`). |
| R5 | Exit-code confusion between `--check` and mutating modes. | 0/2/3 mean the same thing in both; 4 is retired rather than rebound; 5/6 are new; §10.4 and PIB-013…PIB-015 pin it. |
| R6 | Unbounded archive growth, or a secret retained forever. | Content-addressed dedupe means only distinct content costs bytes; blobs are the artifacts' own sizes, capped by `MaxArtifactBytes`; `tpatch feature intent-archive purge` bounds it explicitly (§9.7); orphans are reported and removable (§9.7.3); the redaction gate refuses secret-shaped content up front (§9.6.1); the committed-history caveat is stated rather than papered over (§9.6.2). |
| R7 | The lock's limited authority is read as full mutual exclusion. | §7.12's concurrency matrix names the unexcluded writers row by row, §7.4.2 states the no-cross-machine and lying-filesystem limits, and §7.4.4 records why every alternative authority was rejected; PIB-104 exercises a concurrent `define`; every entry is CAS-gated on publish and on undo (§7.6.3). |
| R8 | The archive is later cited as provenance. | §9.8's table, ADR-035 D9, the extension of the forbidden-inference list (PIB-143), the `notes`-is-a-hint rule (§12.3.2) and the over-claim guard (PIB-155). |
| R9 | A future reviewer assumes a test proves semantics because it exists. | §18.1's disqualifying assertion shapes; §18.51's sensitivity and semantic-fixture requirements over every guard row. |
| R10 | Blob files confuse `git status` / `land` staging for users. | They are ordinary files under `artifacts/`, swept by the shipped feature-path-set rule (`internal/cli/land.go:723-725`); PIB-152 asserts `land` stages them like any other artifact and PIB-153 asserts `record`'s canonical patch is unaffected. |
| R11 | Doctor accidentally becomes a competing lock client. | D9 never opens or flocks the workspace root; it reports persistent evidence only (§12.5), so it can neither perturb a mutating prepare nor diagnose a second doctor. rev-4's transient probe is removed, and PIB-470 asserts zero open/flock calls with a live holder. |
| R12 | A false-positive `home-absolute-path` or broad `email-pii` match blocks a legitimate regeneration. | The trade is stated in §9.6.1: an edit and a re-run versus a credential in every future clone. The refusal names the artifact and class codes so the operator can see what matched; Q9 records a scoped-override design, deliberately not in v1. |
| R13 | The new prepare authority accidentally changes `feature resource *`. | S1b does not extract `rescap`; PIB-286, PIB-287 and PIB-432 pin its untouched files, behavior, refusal text and platform envelope. |
| R14 | An operator assumes `--abandon-transaction` fixed their files. | Its report states in one line that it moved control state only and changed no canonical file; §6.6 and PIB-270 pin it; the human output names the evidence directory rather than claiming a repair. |
| R15 | A journal lost to `git clean`/fresh clone is read as recoverable or diagnosable. | §7.11.1 states the evidence-loss boundary honestly: `doctor` reports residue but cannot infer loss from ordinary partial bytes; docs disclose the undetectable clean-loss case (PIB-321…PIB-323, PIB-407). |
| R16 | An operator on an unsupported platform, a denied filesystem or a broken `flock` reads the exit-3 refusal as a permanent block on a slug that has pending evidence. | §6.6's total pre-abandon gate table gives rows 5, 6 and 8 a named, repo-relative last-resort procedure and states what it costs, and gives row 7 (contention) wait-and-retry instead, because a live holder's undo evidence must not be deleted under it; §10.4 qualifies the abandon half of the exit-6 partition by that table rather than leaving an unconditional claim; PIB-495 asserts the message, PIB-511 asserts the table is total, PIB-512 asserts the contention wording and PIB-496 fails any unqualified "always recoverable" sentence in this cluster. |
| R17 | An operator or harness assumes a mutating run did the work when it actually recovered. | The outcome token is a distinct closed value (`recovered`), `action` is `none`, the advisory says the requested operation was not performed, the `--quiet` line ends in `recovered`, and the report hands over the exact retry (§7.11, §10.2, §10.6, PIB-483, PIB-485). |
| R18 | An operator hits `archive-purge-evidence-divergent`, is told to run `--abandon-transaction`, and finds it refuses `no-pending-transaction` — leaving them with two refusals and no route. | rev-7 partitions the exit-6 remediation by population: that code is routed **only** to §9.7.2's archive procedure, which names the pending hash, the repo-relative managed blob and index paths, the destructive warning, the type-total removal step (blob form) or the restoration step (index form) and the sanitized rerun whose pending+absent recovery terminally tombstones; the message is forbidden from naming abandon. PIB-506 asserts the message, PIB-507 asserts the procedure actually finishes the purge, and PIB-508 fails any mapping that sends a code to a route that cannot consume it. |
| R19 | An operator reads the `archive-purge-partial` report as "one more command finishes it" when their branch needs two, or waits for a `recovered` outcome that will never come. | The closed `purge_progress.resume` field names the branch, `pending_hash` is present exactly on the branch that recovers first, and the prose is written per branch rather than once (§9.7.2, §10.4). PIB-516 and PIB-517 exercise the two no-recovery branches end to end, and PIB-518 fails a report whose shape and promise disagree. |
| R20 | An operator whose archive holds a blob beside a tombstone is sent to the pending-hash escape, finds no pending hash to name and no rerun that finalizes anything, and is left with an unexecutable procedure. | The state is never divergence: X11 refuses exit 3 `archive-index-storage-inconsistent` with zero writes and names the one executable repair for its subcase — `purge <slug> --orphans --yes` when the hash is globally unreferenced, admitted past that observation after the strict wire decode and removing the blob without rewriting the index (§9.3, §9.3.1, §9.7.3). PIB-521 asserts the refusal and its route, PIB-522 asserts the repair actually clears it, PIB-523 asserts the preview reports it, and PIB-524 fails any classification that files it back under `archive-purge-evidence-divergent`. |
| R21 | Two commands own the pending-purge transaction, so `archive-purge-evidence-divergent` can be emitted by a `prepare` whose output cannot carry the archive procedure that resolves it. | The transaction has exactly one owner: `RecoverPendingPurge` has a single call site, on `feature intent-archive purge --yes`. A mutating `prepare` that observes `purge_pending` refuses exit 3 `recovery-pending` with zero writes and hands over the purge command with `retry_cwd` (§7.8 step 5, §7.11, §10.4.1). PIB-525 asserts the refusal and its zero writes, PIB-526 fails any second call site. |
| R22 | A tombstoned reference is read as proof that its blob is unreferenced, `--orphans --yes` deletes a blob another generation still retains, and the operator loses recoverable bytes plus gains a dangling reference — through the one selector that promises to remove nothing referenced. | Liveness is a **global** property of the content hash and is computed over every generation before anything is classified or selected (§9.3). The orphan predicate excludes any hash with a retained or removal-pending reference; the mixed state gets its own classification and its own confirmed `--blob <h> --yes` repair (§9.3.1 subcase B, §9.7.1, §9.7.3). PIB-531 asserts the classification across all four surfaces, PIB-532 asserts the repair restores global truth, PIB-535 pins `list`'s exits and remediation for both subcases, and PIB-533 is the guard whose sensitivity fixture is exactly a per-reference predicate — valid code that deletes live data — which must fail. |
| R23 | The purge recovery itself creates a dangling reference: it removes `h.blob` because *some* reference to `h` is removal-pending, while another generation's reference to `h` is still retained. The archive's repair mechanism becomes a producer of the state it repairs. | A removal-pending reference makes the purge transaction the **global owner** of the hash (§9.3). The recovery claims `h` globally — one CAS rewrite making every reference to `h` removal-pending, taken only while the blob is present and hash-correct — revalidates immediately before the removal, and **removes only when no reference to `h` is anything but pending**. Where the blob is already absent it tombstones every reference instead, which is the shipped dangling repair. PIB-539 asserts the invariant with an index-write spy over the claim/remove/tombstone order, PIB-540 drives the crash points, and PIB-544 drives the four external-insertion windows (§9.7.2, §7.10 CP12/CP12a). |
| R24 | A `purge --orphans --yes` removes the residue it was asked about while a mixed tombstone/live-reference hash elsewhere in the same index goes unmentioned, leaving an archive that is partly repaired and whose next observation is different from the one just acted on. | X11's storage observation is over the **whole index** before every archive mutation, independent of the selector (§9.3.1). A global inconsistency refuses every selector at exit 3 with the whole tree byte-identical, naming that observation's owning repair; each admitted repair is admitted only for the exact inconsistency it repairs, after global classification. PIB-542 asserts a disjoint-selector refusal with a whole-tree snapshot, and `list`/`doctor` render every observation at once so one pass shows the whole inventory (PIB-541). |
| R25 | An operator finds a present but hash-wrong blob under a retained reference, gets a refusal with no route, and either edits the index by hand or deletes the archive. | The observation is `archive-blob-corrupt` at exit 3, zero-write, pinned across `list`, `doctor` and every ordinary mutation, with one repo-relative procedure: one type-total `rm -rf --` of the managed blob path under an explicit destructive warning, then the confirmed `purge <slug> --blob <h> --yes` — which by then is the ordinary, already-admitted dangling repair — or restore the exact correct blob and retry. The cost and the Git-history caveat are stated. PIB-543 asserts the classification, the route and the end-to-end composition, and PIB-547 asserts the printed command executes on all five object kinds (§9.3.1, §9.7.3, §10.7). |
| R26 | A shipped refusal prints `rm -rf`, an operator pastes it in the wrong directory or against a wrong path, and loses unrelated work. | The form is constrained rather than trusted: it names **one** repo-relative managed `blobs/<hash>.blob` path that X11 has already validated as contained in the feature's archive, carries the `--` terminator so a leading `-` cannot become an option, contains no wildcard, no directory prefix above the blob and no second path, and is printed beneath an explicit statement that it deletes whatever is there — a directory and its contents included — with no undo. It is one of only three permitted external shell forms (§10.7), and PIB-547 asserts the exact shape, the warning's position, the absence of a wildcard and the fact that a symlink's target is untouched. The alternative — printing a copy command first — is what rev-11 withdrew, because it was wrong for four of the five object kinds and gave false confidence before a destructive step (§9.3.1, §9.7.2). |
| R27 | An operator's archive holds two instances of one repair class and every selector refuses, so the shipped repair is unreachable and the archive is bricked. | Admission is per repair **class** with full coverage rather than per instance: `--orphans --yes` clears every unreferenced residue in one run, and repeated `--blob` (or an applicable `--all`) clears every same-class hash. `list` and `doctor` render multiple instances of one class as one class with many instances carrying a single invocation. PIB-548 asserts the three multi-instance fixtures end to end and PIB-549 fails an admission predicate that keeps rev-10's sole-inconsistency rule (§9.3.1, §9.7.1, §9.7.3). |
| R28 | The whole-index validation added to protect the archive blocks the recovery of a purge transaction that is already half applied, leaving a pending hash no command can finish. | Completing an already-pending purge is the one explicit exception to pre-mutation global validation: `RecoverPendingPurge` runs before the global scan, finalizes only hashes the transaction already owns, is terminal at exit 0 `recovered`, and never proceeds into the selector; the operator's rerun is then an ordinary new-selector invocation fully governed by the scan. PIB-546 derives the ordering from control flow and fails a scan-first implementation, a recovery that continues into the selector and a `prepare` granted the same exception (§9.3.1, §9.7.2, §10.5 step 22). |

## 17. Implementation slices and file ownership

All slices are gated on §19. Each is independently reviewable.

### 17.1 The prerequisite that must land first

**S0 is not a slice of this cluster; it is a precondition on another one.**

The accepted [PRD-artifact-validation-and-provenance](./PRD-artifact-validation-and-provenance.md)
is **accepted planning, not shipped code**: `prepare --check` does not exist in
any binary yet, and its slices S1–S5 have never been dispatched. Every slice
below builds on it — the inspector, the slug grammar, the state enum, the
report routing, the `internal/cli/prepare.go` file itself. Three rules follow
and they are hard:

1. **No mutating slice may be dispatched until the accepted `--check`
   implementation has landed on `origin/main` and its own acceptance matrix
   passes.** Not "is in flight", not "is in the same cluster": landed and
   green.
2. **Its goldens must be produced by its own implementation, not by this
   cluster.** A golden captured from a binary this cluster built, and then
   compared against a binary this cluster built, proves nothing — it is
   precisely the no-op-versus-no-op comparison the accepted contract warns
   about (`docs/prds/PRD-artifact-validation-and-provenance.md:3515-3520`).
   PIB-198 … PIB-207 compare against goldens committed by the `--check`
   implementation cluster, and PIB-391 asserts the golden fixtures' provenance
   is that commit range rather than this one.
3. **S1b — rescap non-invalidation.** S1b does not move
   `internal/rescap`'s file lock. Its existing goldens, including contention
   and unsupported-platform refusal text, remain byte-identical; PIB-286,
   PIB-287 and PIB-432 enforce that parity. `internal/rescap/gitgate.go` is
   touched by **S4** and only to route its Git spawns through the central
   executor behind an explicit compatibility wrapper that preserves its
   current environment, argv asymmetries and output (§7.13, PIB-472,
   PIB-473).

If (1) is not satisfied at dispatch time, the cluster lead does not dispatch:
there is no partial ordering that makes a mutating slice safe on top of an
unimplemented read half.

### 17.2 Slices

| Slice | Scope | New/modified files |
|---|---|---|
| **S1** | Transaction core: journal schema + strict decoder and J1–J10 binds, identity, semantic CAS, the rooted durable single-file control writer of §7.7.1 and the rest of the rooted write primitives, staging, revalidation, publication order, rollback, recovery, cleanup. Pure package, no CLI. | new `internal/intentpub/**` |
| **S1b** | Prepare-owned directory authority: one `WorkspaceAuthority` retaining `*os.Root` **and** `Root.Open(".")`'s `*os.File`; `SyscallConn.Control` flock/unlock with the single-goroutine ownership rule (§7.4.1), explicit close + `runtime.KeepAlive`, `fstatfs`-on-held-descriptor classification against the exact denied lists (§7.4.2), and identity/live-original-path revalidation. No doctor probe seam exists. This is **not** extraction of `rescap`'s file lock; rescap lock behavior is byte-identical. | new `internal/intentlock/**`; tests beside it |
| **S2** | Generator extraction: `GenerateAnalysis`/`GenerateSpec`/`GenerateExploration`, in-memory-only retry response handling and the refactor of `RunAnalysis`/`RunDefine`/`RunExplore` to call them. **Lands the pre-change goldens for `analyze`/`define`/`explore`/`cycle`/`next` first.** | modified `internal/workflow/workflow.go`, `internal/workflow/retry.go`; new `internal/workflow/generate_*.go` |
| **S3** | The archive: immutable digest-bearing tombstones, X1–X11 strict/storage decode, global-hash rehydration of tombstoned/pending references, redaction-before-rehydrate, deterministic pending → global claim → remove/recover → tombstone-every-reference, complete selector-independent purge preflight and the `archive-purge-partial` outcome, shared references, the single confirmed-purge dangling repair, the retained corrupt-blob route and orphan handling. | new `internal/store/intent_archive.go` |
| **S4** | CLI wiring: modes, rooted manual status helper, flag mutexes plus the command-owned `--yes` preflight of §10.5 step 1a, the dry-run branch of §10.5 step 7, the abandon-before-Git-and-recovery branch of §10.5 step 10, the terminal-recovery returns of §10.5 steps 12–13, report model (including `execution_preflight`, `recovery`, `retry_cwd` and `purge_progress`), renderers, exits, advisories and `--abandon-transaction`. It owns the **one** closed G1–G4 executor/refactor: G1 tri-state is established once and threaded with repo-relative lane paths; the pinned environment scrub of §7.13 applies to prepare's own invocations; every existing caller keeps an explicit compatibility wrapper and its goldens. | modified `internal/cli/prepare.go` (the file the accepted S3 creates), new `internal/cli/prepare_publish.go`; modified `internal/gitutil/ignore.go`, `internal/workflow/session_ignore.go`, `internal/rescap/scratch.go`, `internal/rescap/gitgate.go`; tests beside each |
| **S4b** | The retention surface: `tpatch feature intent-archive list\|purge`. | new `internal/cli/feature_intent_archive.go`; modified `internal/cli/feature_deps.go` (the group registration line only) |
| **S5** | `doctor` D9 — persistent-evidence reporting only, with **no** root open and no flock probe; compatibility and non-invalidation proofs. D1–D8 retain their CLI goldens. | new `internal/workflow/doctor_d9.go`; modified `internal/workflow/doctor.go` (registry only) and `internal/cli/doctor.go` (D9 help/check-list only) |
| **S6** | Docs, six skill surfaces, parity-guard extension, over-claim and citation guards, sensitivity meta-check. | `SPEC.md`, `docs/**`, `assets/skills/**`, `assets/assets_test.go` |
| **S7** | Rev-4 … rev-11 cross-cutting hardening after owned slices: terminal-recovery ordering and its exit-3 zero-write consequence, purge's journal-marker refusal and its purge-owned terminal pending-hash recovery on every selector, `prepare`'s zero-write `recovery-pending` refusal on a removal-pending index, the globally-evaluated tombstone-beside-blob split with its `--orphans --yes` and confirmed `--blob <h> --yes` repairs, the one verbatim retry heading over every emitter, the classified root-open/`Open(".")` refusal, the bound pending-purge preview shape, abandon reachability with `git` absent/unusable and with an absent/malformed/unreadable feature, the total pre-abandon gate table over every argv requesting a true abandon (or failing parse while naming the flag) and its wait-and-retry contention case, the global pending-hash ownership invariant and its total same-hash claim-then-remove-then-tombstone order, the selector-independent whole-index storage validation with its zero-write refusal and its one pending-recovery ordering exception, the per-repair-class multi-instance admission, the type-total corrupt-object removal route and the disclosed revalidate-to-unlink residual, the partitioned exit-6 route map and the archive-divergence procedure, the three conditional partial-purge branches, the purge Git zero and the purge-preview lock zero, sanitized retries and repeat-abandon residue, the command-owned `--yes` literal, `Control`-scoped `fstatfs` with no `Fd()` in the authority path, forced-GC real-process authority lifetime, release-then-`Control` determinism, alias-at-acquisition/renamed-original-path refusal, `fstatfs`-on-held-descriptor and exact denied-class fixtures, Linux/macOS root-inode contention/release, abandon-before-recovery ordering with corrupt/divergent/recoverable evidence, rooted control-write proofs, purge preflight/partial-outcome/crash recovery and the single dangling repair, structural raw-sink and schema-scoped provenance guards, closed Git count/argv/environment spies including the indexed config forms, dry-run branch totality, and D9 zero-probe proofs. | tests beside `internal/intentlock`, `internal/intentpub`, `internal/store`, `internal/cli`, `internal/workflow`, `internal/gitutil`, `internal/rescap`; no new public surface |

**Ordering.** S1b → S1 → S3 → S4 → S4b is strict. S2 may run in parallel with
S1/S1b/S3 **only** under an explicit file partition; S5 and S6 follow S4b;
**S7 is last and sequential** because it exercises every revised surface.

**Parallel-implementer discipline.** `internal/cli/prepare.go`, `internal/workflow/workflow.go`,
`internal/workflow/doctor.go`, `internal/cli/doctor.go`,
`internal/gitutil/ignore.go`, `internal/workflow/session_ignore.go`,
`internal/rescap/scratch.go` and `internal/rescap/gitgate.go` are
shared surfaces. Per AGENTS.md, same-file
overlap is a hard trigger for sequential execution: **no two implementers may
touch any one of those files.** The cluster lead must declare the partition at
dispatch, every implementer stages by explicit path, and `git commit -a`,
`git add .`, `git add -A` and directory-scope adds are forbidden for this
cluster.

**Golden prerequisite.** S2's pre-change goldens for `analyze`, `define`,
`explore`, `cycle`, `next` and `doctor` D1…D8, must be captured and committed **before** the
corresponding refactor lands, or PIB-186, PIB-208…PIB-212 and PIB-286…PIB-287
degrade into comparing a changed binary against itself.

## 18. Acceptance matrix

### 18.1 How to read this matrix

IDs are stable (`PIB-NNN`) and are never renumbered; a retired row is struck
through and keeps its number. Each row names the **observable** the test must
assert.

**A test does not satisfy a row merely by existing.** A row is satisfied only
when its test asserts the named observable. Disqualifying assertion shapes,
inherited from the accepted contract
(`docs/prds/PRD-artifact-validation-and-provenance.md:3531-3548`):

- asserting "the command did not return an error" satisfies no row;
- asserting "some JSON was produced" satisfies no row that names a field,
  value, order or exit code;
- a row naming an exit code must assert the numeric **process** exit code, not
  the presence of a Go error;
- a row naming "byte-identical" must compare bytes, not lengths, sizes or
  mtimes;
- a row naming a state, code or enum must assert that exact literal, not a
  truthy proxy;
- a row naming a **guard** must assert the guard's semantics — that it fails on
  a wrong input — not that the guard function or its test file exists;
- a row naming "zero writes" must be asserted with a filesystem spy or a
  whole-tree byte snapshot, never by re-reading one file.

If a row cannot be placed as written, the PRD is amended — the row is not
silently re-tiered.

Legend for **Kind**: `U` unit, `I` integration (real CLI invocation over a real
temp workspace), `S` source scan (AST), `G` mechanical guard, `C` crash/fault
injection through the declared seams.

**Injection seams.** Crash and failure rows are driven through named,
production-inert seams: `beforeLockAcquire`, `failLockAcquire`,
`beforeAbandonBranch`, `beforeAbandonMove`, `afterAbandonMove`,
`beforeRedactionScan`, `beforeBlobWrite`, `afterBlobWrite`,
`beforeJournalWrite`, `afterJournalWrite`, `beforeControlWriteRename`,
`beforeEntryCAS(i)`,
`beforeRename(i)`, `afterRename(i)`, `beforeIndexRewrite`,
`beforeStatusRename`, `afterStatusRename`, `beforeFinalVerify`,
`beforeJournalClear`, `beforeLockRelease`, `afterLockRelease`,
`beforeBlobRemove`, `afterPurgeBlobRemove`,
`beforePendingTombstoneCAS`, `failPurgeAfterFirstMutation`,
`beforeManualStatusCAS`, `beforePurgeIndexCAS`, `afterPurgeIndexRename`,
`beforeRehydrateIndexRename`, `beforeRootIdentityCheck`,
`beforePurgeBlobRemove`, `afterPurgeBlobRevalidate`,
`failPurgeBetweenHashes`, `failOrphanRemoveAfterFirst`,
`afterRecoveryComplete`, `failFsync(path)`,
`failRename(path)`. Each is a function-valued package
variable that is `nil` in production; PIB-232 asserts every one is `nil` at
init and that no production call path assigns one. rev-4's
`beforeDoctorLockProbe` is **removed** together with the probe it drove
(§12.5). rev-6 adds `afterRecoveryComplete`, which is what lets PIB-484 observe
that nothing runs between a completed recovery and the command's return. rev-7
adds `failPurgeBetweenHashes` and `failOrphanRemoveAfterFirst`, which are what
let PIB-516 and PIB-517 reach the two partial-purge branches that carry **no**
pending marker and therefore cannot be produced by `failPurgeAfterFirstMutation`.
rev-8 adds **no** seam: every state it classifies or re-routes is reachable
through the existing set, `beforeRehydrateIndexRename` in particular being what
produces CP13's unreferenced residue. rev-9 adds **no** seam either: the mixed
tombstone/live-reference state is an index fixture rather than a crash point,
and `beforeLockAcquire` — which already exists — is what PIB-536 uses to make
the root-open and `Open(".")` failures deterministic. rev-10 adds **no** seam
either: the global claim's crash points and its four external-insertion windows
are reached through `beforePurgeIndexCAS`, `afterPurgeBlobRevalidate`,
`beforePurgeBlobRemove`, `afterPurgeBlobRemove` and
`beforePendingTombstoneCAS`, all of which already exist, and every other rev-10
state is an index or filesystem fixture rather than a fault. rev-11 adds **no**
seam either: PIB-550's revalidate→unlink replacement is injected at the existing
`beforePurgeBlobRemove` after `afterPurgeBlobRevalidate` has passed, and every
other rev-11 state — the tombstoned-reference claim, the multi-instance repair
classes, the five corrupt object kinds and the compound recovery ordering — is
an index or filesystem fixture, a control-flow derivation or a printed string
rather than a fault.

**Amended rows.** IDs are never renumbered. Rev-1 amended `PIB-015`,
`PIB-016`, `PIB-017`, `PIB-020`, `PIB-041`, `PIB-053`, `PIB-079`, `PIB-110`,
`PIB-117`, `PIB-118`, `PIB-124`, `PIB-196`, `PIB-197`, `PIB-215`, `PIB-227`.
Rev-2 additionally amended `PIB-051`…`PIB-053`, `PIB-079`, `PIB-080`,
`PIB-103`, `PIB-117`, `PIB-118`, `PIB-124`, `PIB-134`, `PIB-135`, `PIB-139`,
`PIB-221`…`PIB-223`, `PIB-279`, `PIB-282`, `PIB-284`, `PIB-285`,
`PIB-288`…`PIB-290`, `PIB-310`, `PIB-312`, `PIB-313`, `PIB-321`…`PIB-323`,
`PIB-327`…`PIB-329`, `PIB-340`, `PIB-349`, `PIB-352`, `PIB-354`,
`PIB-380`, `PIB-381`, `PIB-386`, `PIB-392`…`PIB-394`.

**Rev-3 explicitly amends**, rather than silently re-meaning,
`PIB-053`, `PIB-079`, `PIB-080`, `PIB-117`, `PIB-124`, `PIB-125`,
`PIB-134`, `PIB-135`, `PIB-188`…`PIB-190`, `PIB-222`, `PIB-223`,
`PIB-268`, `PIB-279`, `PIB-282`, `PIB-284`, `PIB-285`,
`PIB-288`, `PIB-290`, `PIB-343`, `PIB-349`, `PIB-356`, `PIB-357`,
`PIB-359`, `PIB-376`, `PIB-380`, `PIB-381`, `PIB-386`,
`PIB-392`, `PIB-393`, `PIB-395`…`PIB-398`, `PIB-403`, `PIB-405`, and
`PIB-409`. Their old cache/Windows/raw-response/shared-reference meanings are
retired; no stable ID has silently changed. Rev-3 adds only `PIB-410`…`PIB-432`.

**Rev-4 explicitly amends**, rather than silently re-meaning,
`PIB-079`, `PIB-080`, `PIB-107`, `PIB-117`, `PIB-124`, `PIB-125`,
`PIB-188`…`PIB-191`, `PIB-268`, `PIB-279`…`PIB-283`, `PIB-285`…`PIB-290`,
`PIB-326`, `PIB-345`…`PIB-361`, `PIB-364`, `PIB-376`, `PIB-380`,
`PIB-381`, `PIB-392`, `PIB-393`, `PIB-395`…`PIB-398`, `PIB-403`…`PIB-405`,
`PIB-409`…`PIB-420`, and `PIB-421`…`PIB-432`. Their old descriptor-lifetime,
rename-success, allowlist, pending-purge, raw-substring, zero-Git and
evidence-only-doctor meanings are retired; no stable ID silently changes.
Rev-4 adds only contiguous `PIB-433`…`PIB-448`.

**Rev-5 explicitly amends**, rather than silently re-meaning, every stable row
whose observable moved with this revision:

- *Dry-run scope* (§6.4, §10.5 step 7): `PIB-072`, `PIB-074`, `PIB-077`,
  `PIB-079`, `PIB-080`, `PIB-268`, `PIB-440`.
- *Abandon reachability and ordering* (§6.6, §7.8, §10.5 step 10): `PIB-015`,
  `PIB-270`…`PIB-274`, `PIB-320`, `PIB-362`, `PIB-363`.
- *Rooted control writes* (§7.5, §7.7.1, §7.7.3, §13.2): `PIB-096`, `PIB-308`,
  `PIB-309`, `PIB-312`, `PIB-313`.
- *Single dangling repair* (§9.3, §9.3.1 X11, §9.7): `PIB-340`, `PIB-356`,
  `PIB-357`, `PIB-402`, `PIB-403`, `PIB-425`, `PIB-428`, `PIB-444`,
  `PIB-447`, `PIB-448`.
- *Purge preflight, partial outcome and exit-3 zero-write* (§9.7.2, §10.4):
  `PIB-347`…`PIB-355`, `PIB-358`, `PIB-359`, `PIB-405`, `PIB-421`, `PIB-422`,
  `PIB-423`, `PIB-429`, `PIB-430`, `PIB-443`.
- *Doctor probe removal* (§12.5): `PIB-133`…`PIB-136`, `PIB-232`, `PIB-380`,
  `PIB-381`, `PIB-386`, `PIB-387`, `PIB-445`.
- *Git surface, scrub and wrappers* (§7.13, §17.2): `PIB-106`, `PIB-107`,
  `PIB-279`…`PIB-283`, `PIB-326`, `PIB-408`, `PIB-427`, `PIB-438`, `PIB-439`.
- *Guard feasibility* (§10.2, §13.4): `PIB-144`, `PIB-147`, `PIB-190`,
  `PIB-376`, `PIB-419`.
- *Authority, filesystem and reference truth* (§7.4.1, §7.4.2, §7.4.4):
  `PIB-286`, `PIB-287`, `PIB-392`…`PIB-394`, `PIB-409`…`PIB-411`, `PIB-416`,
  `PIB-418`, `PIB-433`, `PIB-435`, `PIB-441`, `PIB-442`.
- *Catalogs, vocabularies and ledgers* (§10.2, §10.4.1, §18.50): `PIB-226`,
  `PIB-228`, `PIB-229`, `PIB-230`, `PIB-231`, `PIB-431`.

Their old dry-run-equivalence, recovery-before-abandon, path-based-journal,
rehydration-as-dangling-repair, zero-write-purge-failure, doctor-probe,
unscrubbed-`gitgate` and byte-scan-provenance meanings are retired; **no stable
ID silently changes meaning, and none is renumbered.** Rev-5 adds only
contiguous `PIB-449`…`PIB-482`.

**Rev-6 explicitly amends**, rather than silently re-meaning, every stable row
whose observable moved with this revision. rev-7 corrected this ledger: rev-6
originally listed `PIB-362` and `PIB-363` here, but neither row's text or
observable was in fact changed by rev-6, so both are removed from this list and
appear in rev-7's instead. The rows rev-6 truly amended are:

- *Terminal recovery* (§7.8 steps 4–5, §7.11, §10.4, §10.5 steps 12–13):
  `PIB-015`, `PIB-113`, `PIB-177`, `PIB-469`.
- *Purge refuses a pending journal* (§9.7, §9.7.2): `PIB-350`, `PIB-467`.
- *Abandon reachability through a broken Git* (§6.6, §7.8 steps 2–3, §7.13,
  §10.5 steps 10–11): `PIB-452`, `PIB-274`.
- *Sanitized retries* (§9.7.2, §10.2): `PIB-466`.
- *Flag grammar* (§5.1, §5.2, §5.3, §10.5 step 1a): `PIB-243`.

Their old recover-then-continue, purge-recovers-the-journal,
abandon-behind-the-lane-gate, verbatim-same-command-retry and
`--yes`-as-a-cobra-mutex meanings are retired; **no stable ID silently changes
meaning, and none is renumbered.** Rev-6 adds only contiguous
`PIB-483`…`PIB-505`.

**Rev-7 explicitly amends**, rather than silently re-meaning, every stable row
whose observable moved with this revision — and only those rows:

- *Exit-6 route partition* (§6.6 rule 8, §9.7.2, §10.4, §10.4.1): `PIB-362`,
  `PIB-363`.
- *Abandon pre-gate totality* (§6.6's gate table, §10.5 steps 6, 9 and 10):
  `PIB-495`.
- *Lane parity and the purge Git zero* (§7.13, §12.6 D9): `PIB-281`,
  `PIB-282`, `PIB-283`.
- *Conditional partial-purge retry* (§9.7.2, §10.2, §10.4.1): `PIB-466`,
  `PIB-467`.
- *Qualified recoverability* (§2.1 goal 5, §10.4): `PIB-291`, `PIB-294`.
- *Grammar-line completeness* (§5.1): `PIB-244`.

Their old single-route-exit-6, three-condition-abandon-gate,
lane-gate-includes-abandon, unconditional-two-run-purge-retry,
unqualified-never-blocked and delta-table-only-completeness meanings are
retired; **no stable ID silently changes meaning, and none is renumbered.**
Rev-7 adds only contiguous `PIB-506`…`PIB-520`.

rev-8 corrected this ledger too: rev-7 originally listed `PIB-274` under
*Abandon pre-gate totality*, but that row's text and observable were changed by
**rev-6**, not by rev-7 — verified by diffing `7af5092` against `751d817`, in
which `PIB-274` appears only in ledger prose and not as a changed matrix row —
so it is removed here and stays in rev-6's list where it belongs. The eleven
rows above are exactly the rows the rev-7 diff changed.

**Rev-8 explicitly amends**, rather than silently re-meaning, every stable row
whose observable moved with this revision — and only those rows, each one
verifiable in the rev-8 diff:

- *Unreferenced physical residue* (§9.3, §9.3.1, §9.7.3, §10.4.1): `PIB-404`,
  `PIB-447`, `PIB-506`.
- *Purge-owned pending recovery* (§7.8 step 5, §7.11, §10.5 step 13):
  `PIB-453`, `PIB-486`.
- *Pending-purge preview shape and selector totality* (§9.7, §10.2):
  `PIB-226`, `PIB-367`, `PIB-515`.
- *Pre-abandon gate reachability* (§6.6's gate table): `PIB-511`.

Their old tombstone-beside-blob-is-divergence, crash-leaves-a-plain-orphan,
prepare-finalizes-a-pending-hash, abandon-names-a-prepare-recovery-route,
single-selector-preview and presence-only-gate-table meanings are retired;
**no stable ID silently changes meaning, and none is renumbered.** Rev-8 adds
only contiguous `PIB-521`…`PIB-530`.

**Rev-9 explicitly amends**, rather than silently re-meaning, every stable row
whose observable moved with this revision — and only those rows, each one
verifiable in the rev-9 diff against `837f28a`:

- *Global-hash residue split* (§9.3, §9.3.1 X11, §9.7, §9.7.1, §9.7.3, §10.4.1,
  §12.5, §7.10 CP13): `PIB-356`, `PIB-404`, `PIB-447`, `PIB-506`, `PIB-521`,
  `PIB-523`, `PIB-524`.
- *One verbatim retry heading* (§9.7, §9.7.2, §10.2): `PIB-498`, `PIB-529`.
- *Pre-abandon gate domain* (§6.6's gate table, §10.5 steps 9 and 10):
  `PIB-511`.

`PIB-498` additionally changes **Kind**, from `I` to `G`, because it is now the
derived totality guard over every retry emitter rather than one path's
assertion; the kind change is carried in §18.50's arithmetic and the row gains a
§18.51 semantic-fixture entry; §18.51's table grows from nineteen to
twenty-two guards (PIB-356, PIB-498 and PIB-533 join it, and nothing leaves).
Their old
per-reference-orphan, single-tombstone-route, two-spelling-retry-heading and
syntactically-valid-domain meanings are retired; **no stable ID silently changes
meaning, and none is renumbered.** Rev-9 adds only contiguous
`PIB-531`…`PIB-536`.

**Rev-10 explicitly amends**, rather than silently re-meaning, every stable row
whose observable moved with this revision — and only those rows, each one
verifiable in the rev-10 diff against `ebd1be8`:

- *Global pending ownership* (§9.3, §9.7.2, §7.10 CP12/CP12a/CP13, §7.8 step 5,
  §7.11): `PIB-356`, `PIB-447`, `PIB-524`, `PIB-529`, `PIB-534`.
- *Selector-independent whole-index X11 validation* (§9.3.1, §9.7.1, §9.7.2,
  §10.5 step 22): `PIB-465`, `PIB-533`, `PIB-534`.
- *Retained corrupt-blob route* (§9.3.1, §9.7.3, §10.4.1, §10.7): `PIB-356`,
  `PIB-428`, `PIB-535`.
- *Retry heading column-0 parity* (§9.7, §9.7.2, §10.2): `PIB-498`, `PIB-529`.
- *Abandon boolean domain* (§5.2, §6.6): `PIB-511`.
- *List/doctor coexistence precedence* (§9.7, §12.5): `PIB-535`.

No row changes **Kind** in rev-10, so §18.50's arithmetic carries the rev-9
totals forward unchanged before the nine new rows are added. §18.51's table
grows from twenty-two to twenty-three guards (`PIB-545` joins it, and nothing
leaves); `PIB-498`, `PIB-511`, `PIB-524` and `PIB-533` each gain an additional
semantic fixture inside their existing entries. Their old
per-reference-recovery, selection-scoped-validation, unclassified-corrupt-blob,
indentation-agnostic-heading, flag-appears-anywhere-domain and
single-observation-list meanings are retired; **no stable ID silently changes
meaning, and none is renumbered.** Rev-10 adds only contiguous
`PIB-537`…`PIB-545`.

**Rev-11 explicitly amends**, rather than silently re-meaning, every stable row
whose observable moved with this revision — and only those rows, each one
verifiable in the rev-11 diff against `a9ad7c0`:

- *Total same-hash claim* (§9.3, §9.7.1, §9.7.2): `PIB-532`, `PIB-545`.
- *Pending-recovery ordering exception* (§9.3.1, §9.7, §9.7.2, §10.5 step 22):
  `PIB-534`.
- *Type-total corrupt-object removal* (§9.3.1, §9.7.2, §10.4.1, §10.7):
  `PIB-506`, `PIB-507`, `PIB-543`.
- *Repair-class multiplicity* (§9.3.1, §9.7.1, §9.7.2, §9.7.3, §12.5):
  `PIB-535`, `PIB-542`.
- *State-map totality and the pinned residual* (§9.3, §9.7.2): `PIB-524`,
  `PIB-544`.

No row changes **Kind** in rev-11 either, so §18.50's arithmetic carries the
rev-10 totals forward unchanged before the six new rows are added. §18.51's
table grows from twenty-three to twenty-six guards (`PIB-546`, `PIB-549` and
`PIB-551` join it, and nothing leaves); `PIB-524` and `PIB-545` each gain
additional semantic fixtures inside their existing entries. Their old
tombstoned-reference-no-op, sole-inconsistency-admission,
`cp`-preservation-procedure, four-window and three-blob-observation meanings are
retired; **no stable ID silently changes
meaning, and none is renumbered.** Rev-11 adds only contiguous
`PIB-546`…`PIB-551`.

### 18.2 A — CLI grammar, modes and flag mutexes

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-001 | I | `prepare <slug>` on a feature missing `spec.md` | exit 0; `spec.md` created; report `outcome: published` |
| PIB-002 | I | `prepare` with no slug | exit 1; cobra arity text; zero writes |
| PIB-003 | I | `prepare a b` | exit 1; zero writes |
| PIB-004 | I | `prepare <slug> --manual --regenerate` | exit 1; cobra mutual-exclusion text; zero writes |
| PIB-005 | I | `prepare <slug> --check --manual` | exit 1; mutual-exclusion text; zero writes |
| PIB-006 | I | `prepare <slug> --check --regenerate` | exit 1; mutual-exclusion text; zero writes |
| PIB-007 | I | `prepare <slug> --check --timeout 5s` | exit 1; mutual-exclusion text |
| PIB-008 | I | `prepare <slug> --manual --timeout 5s` | exit 1; mutual-exclusion text |
| PIB-009 | I | `prepare <slug> --manual --no-retry` | exit 1; mutual-exclusion text |
| PIB-010 | I | `prepare <slug> --check --dry-run` | exit 1; mutual-exclusion text |
| PIB-011 | I | `prepare <slug> --check --manual` | zero mutation: whole `.tpatch/` tree byte-identical |
| PIB-012 | I | `prepare <slug> --check --regenerate` | zero mutation: whole `.tpatch/` tree byte-identical |
| PIB-013 | I | `prepare <slug>` (no `--check`) in a workspace | exit is **not** 4; a real run occurs |
| PIB-014 | G | source scan over `internal/cli/**` | the retired reserved-surface refusal string is absent, and no code path constructs `ExitCodeError{Code: 4}` in the `prepare` command file |
| PIB-015 | I | table over all seven codes | each of 0,1,2,3,5,6 is reachable by a named input, including exit 0 for a terminal `recovered` outcome, exit 1 for the command-owned `--yes` literal, exit 3 for lock contention, exit 5 for both an aborted publication **and** `archive-purge-partial`, and exit 6 for an undo-CAS refusal; 4 is unreachable by every input in the table |
| PIB-016 | I | `prepare --help` | lists all five modes; states it is unrelated to `apply --mode prepare`; names `--regenerate` as the only overwrite route and `--allow-heuristic` as its only downgrade opt-in |
| PIB-017 | I | `apply --help` | `--mode` description still points at `prepare`; text updated for five modes |
| PIB-018 | I | `prepare <slug> --json --quiet` on success | stdout is exactly one JSON document; stderr empty |
| PIB-019 | I | `prepare <slug> --quiet` on success | stdout is exactly one line ending in the outcome token |
| PIB-020 | S | the `prepare` command's flag registration | exactly twelve flags registered: `check`, `manual`, `regenerate`, `abandon-transaction`, `allow-heuristic`, `dry-run`, `yes`, `json`, `quiet`, `timeout`, `timeout-phase`, `no-retry`, plus the inherited persistent `path`; no `--all`, `--fix`, `--force`, `--restore`, `--format`, `--interactive` |

### 18.3 B — Default mode: preservation and missing-only

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-021 | I | all three Markdown files missing | all three created; exit 0; state `defined` |
| PIB-022 | I | `analysis.md` present-nonempty, other two missing | `analysis.md` **byte-identical** afterwards; other two created |
| PIB-023 | C | same as PIB-022, with a write spy | zero `OpenFile`-for-write, zero rename targeting `analysis.md` |
| PIB-024 | I | all three present-nonempty, state `defined` | zero bytes written anywhere in `.tpatch/`, including `status.json` |
| PIB-025 | U | `spec.md` present-empty (zero bytes) | refusal `artifact-empty-not-overwritten`, exit 2 |
| PIB-026 | U | `spec.md` whitespace-only | same as PIB-025 (whitespace-only is empty per the accepted rule) |
| PIB-027 | U | `spec.md` is a symlink to an in-repo file | refusal `artifact-unsafe`, exit 3; symlink not followed; target string absent from all output |
| PIB-028 | U | `exploration.md` is a directory | refusal `artifact-unsafe`, exit 3 |
| PIB-029 | U | `analysis.md` is a FIFO | refusal `artifact-unsafe`, exit 3; no open of it |
| PIB-030 | I | `analysis.md` + `spec.md` present, `exploration.md` absent | only `exploration.md` is created; the other two and the sidecar byte-identical |
| PIB-031 | U | `spec.md` mode 0000 | refusal `artifact-unsafe`, exit 3 |
| PIB-032 | I | Path A feature: `analysis.md` preserved, sidecar present | sidecar byte-identical afterwards |
| PIB-033 | I | Path B feature: `analysis.md` preserved, sidecar absent | sidecar still absent; advisory `analysis-preserved-sidecar-untouched` present |
| PIB-034 | I | zero-byte `spec.md` reached `defined` via `define --manual` | `prepare` refuses exit 2; `spec.md` still zero bytes |
| PIB-035 | I | complete bundle, state `defined` | `outcome: no-op`, `action: none`, exit 0, `status.json` byte-identical (including `updated_at`) |
| PIB-036 | I | complete bundle, state `analyzed` | `action: adopt`; only `status.json` changes; the four artifacts byte-identical |
| PIB-037 | U | `analysis.md` oversize (cap + 1) | refusal `artifact-unsafe`, exit 3; the file is never opened |
| PIB-038 | C | `spec.md` grows during its capture | classified `unstable`; refusal `artifact-unstable`, exit 3 |
| PIB-039 | U | sidecar is `invalid-structured`, all Markdown present-nonempty | no refusal; sidecar untouched; exit 0 with `action: none` |
| PIB-040 | U | sidecar is a symlink, all Markdown present-nonempty | no refusal (optional artifact never gates admissibility); sidecar untouched |
| PIB-041 | I | `analysis.md` generated this run and the sidecar is `absent` | the sidecar is created in the same publication; with a sidecar present in any state the run refuses instead (PIB-253) |
| PIB-042 | I | `analysis.md` preserved, `spec.md` generated | sidecar **not** in the publication set; a rename spy records none targeting it |
| PIB-043 | I | generated `spec.md` | its provider prompt contains the preserved `analysis.md`'s bytes (context flow) |
| PIB-044 | G | the §6.1 disposition table | its Go representation covers all nine accepted enum values with no `default` fallthrough |

### 18.4 C — `--manual` adoption

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-045 | I | `--manual` on a ready bundle, provider spy installed | zero provider calls |
| PIB-046 | I | `--manual` on a ready bundle | all four artifact files byte-identical afterwards |
| PIB-047 | I | `--manual` with `exploration.md` absent | exit 2; full per-artifact report; zero mutation |
| PIB-048 | I | `--manual` with zero-byte `spec.md` | exit 2; zero mutation |
| PIB-049 | I | `--manual` success with `FEATURES.md` unwritable | exit 0; `status.json` written; the index-refresh failure does not fail the command |
| PIB-050 | I | `--manual` success | `state=defined`, `last_command=prepare`, notes equal the frozen adoption string |
| PIB-051 | C | `--manual` success, rooted rename spy | exactly one rooted temp→`status.json` rename after immediate preimage CAS |
| PIB-052 | C | `--manual` success, journal/archive spy | no journal or archive directory is created; the rooted single-file CAS remains required |
| PIB-053 | I | `--manual` while any mutating prepare/purge holds the workspace directory authority | exit 3, `transaction-in-progress`; zero mutation; the remediation says to wait and retry |
| PIB-054 | I | `--manual` with `analysis.md` symlinked | exit 3, `artifact-unsafe`; zero mutation |
| PIB-055 | I | `--manual` from state `requested` with a complete bundle | exit 0; advisory `feature-state-below-defined`; state `defined` |
| PIB-056 | I | `--manual` on a feature already `defined` with a complete bundle | exit 0; `status.json` rewritten only if a field actually changes, else `no-op` |
| PIB-057 | I | `--manual --json` refusal | report carries `mode: manual`, `outcome: refused`, and the per-artifact rows |
| PIB-058 | I | `--manual` with `status.json` malformed | exit 3, `status-malformed`; zero mutation |
| PIB-059 | S | the `--manual` code path | contains no provider construction and no artifact write call |

### 18.5 D — `--regenerate` and the archive

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-060 | I | `--regenerate` on a full hand-authored bundle | each prior file's exact bytes exist at `intent-archive/blobs/<sha256>.blob` |
| PIB-061 | I | `--regenerate` with `spec.md` symlinked | exit 3; **nothing** archived, nothing regenerated, nothing renamed |
| PIB-062 | C | `--regenerate` with the blob write failing (`failFsync`) | exit 5 or 3; zero canonical mutations |
| PIB-063 | I | `--regenerate` where a blob file already exists with equal bytes | it is not rewritten (mtime and inode unchanged); advisory `archive-blob-reused` |
| PIB-064 | I | `--regenerate` where a blob file exists with **different** bytes for its name | exit 3, `archive-blob-corrupt`; nothing overwritten |
| PIB-065 | U | `generation_id` collision, injected through the derivation seam | exit 3, `archive-generation-id-collision`; index unchanged |
| PIB-066 | I | `--regenerate` with an `unreadable` required artifact | exit 3; nothing archived, nothing published |
| PIB-067 | I | `--regenerate` over a zero-byte `spec.md` | the zero-byte file is archived; new `spec.md` non-empty; exit 0 |
| PIB-068 | I | a workspace that never regenerates | no `intent-archive/` directory exists anywhere |
| PIB-069 | I | `--regenerate` twice with identical prior content | the second appends **no** index entry; advisory `archive-generation-duplicate` |
| PIB-070 | I | `--regenerate` | all three Markdown files **and** the sidecar are replaced; no partial-scope option exists |
| PIB-071 | I | `--regenerate` success report | names each archived blob and prints the `cp` restore form verbatim |

### 18.6 E — `--dry-run`

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-072 | C | `--dry-run` in every mode, filesystem spy | zero `mkdir`, `create`, `write`, `rename`, `remove` calls of any kind |
| PIB-073 | I | `--dry-run`, provider spy | zero provider calls |
| PIB-074 | I | `--dry-run` on an admissible plan | exit 0; `outcome: planned`; `dry_run: true`; `execution_preflight: "not_evaluated"` |
| PIB-075 | I | `--dry-run` on a `present-empty` required artifact (default mode) | exit 2, same refusal code as the real run |
| PIB-076 | I | `--dry-run --manual` on an incomplete bundle | exit 2, same refusal code as the real run |
| PIB-077 | I | `--dry-run` report | contains the verbatim plan-only sentence of §6.4, including the execution-preflight clause; carries `execution_preflight: "not_evaluated"`; contains no `generator` value and no archive hash |
| PIB-078 | I | `--dry-run --regenerate` | lists every artifact it would archive; creates no archive directory |
| PIB-079 | C | `--dry-run` with a pending journal | exit 3, `recovery-pending`; recovery does **not** run; the journal is byte-identical; no plan is printed and no Git process, mutating platform/filesystem gate or directory flock is acquired |
| PIB-080 | C | `--dry-run`, authority spy | no mutating `Root.Open(".")` authority path, `SyscallConn.Control` or `Flock` is called |

### 18.7 F — Staging, validation and no partial canonical write

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-081 | C | generation of the 2nd of 3 artifacts fails hard | zero canonical files created or modified; whole `.tpatch/features/<slug>/` byte-identical |
| PIB-082 | C | staged `spec.md` is empty (V1) | exit 2; zero canonical mutations; staging retained |
| PIB-083 | C | staged `spec.md` is whitespace-only (V1) | exit 2; zero canonical mutations |
| PIB-084 | C | staged `analysis.md` exceeds the cap (V2) | exit 2; zero canonical mutations |
| PIB-085 | C | staged bytes contain a NUL (V3) | exit 2; zero canonical mutations |
| PIB-086 | C | staged bytes are invalid UTF-8 (V4) | exit 2; zero canonical mutations |
| PIB-087 | C | staged sidecar is `[1,2,3]` (V5) | exit 2; zero canonical mutations |
| PIB-088 | C | staged sidecar is `{` (V5) | exit 2; zero canonical mutations |
| PIB-089 | C | a staged file changes identity after fsync (V6) | exit 5; zero canonical mutations |
| PIB-090 | I | successful run | the staging tree no longer exists and its parent is empty of `stage-*` |
| PIB-091 | I | failed run | the staging tree still exists; the report names its repo-relative path; advisory `staging-retained` |
| PIB-092 | I | a second successful run after a failed one | the earlier retained staging tree is removed |
| PIB-093 | I | staging tree permissions | directory `0700`; files `0600` |
| PIB-094 | I | `.tpatch/local/` not covered by an ignore rule | exit 3, `local-lane-not-ignored`, before any staging byte |
| PIB-095 | I | a tracked file exists under `.tpatch/local/` | exit 3, `local-lane-not-ignored`; zero mutation |
| PIB-096 | C | staged files after generation | each is written through the §7.7.1 rooted control writer, fsynced, and its identity recorded before the journal is written |
| PIB-097 | S | the staging path builder | derives from the validated slug and `RandomHex12` only; never from provider output |
| PIB-098 | I | run that fails after staging | staging retained; canonical tree byte-identical |
| PIB-099 | I | run that succeeds | staging removed; parent directory fsynced |

### 18.8 G — Publication, revalidation and concurrency

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-100 | C | an external writer modifies `spec.md` between preflight and revalidation | exit 5 before the first rename; zero canonical mutations; staging retained |
| PIB-101 | C | an artifact appears where preflight saw `absent` | exit 5; the appeared file is byte-identical afterwards |
| PIB-102 | C | an artifact disappears between preflight and revalidation | exit 5; zero canonical mutations |
| PIB-103 | I | documented limit: a write landing inside the window | test asserts the shipped docs and messages state the limit; **no row claims the write is preserved** |
| PIB-104 | I | a concurrent `tpatch define` completing before revalidation | exit 5; `define`'s bytes preserved |
| PIB-105 | I | `.tpatch/` replaced wholesale between preflight and revalidation | exit 5 or 3; nothing published |
| PIB-106 | I | any successful `prepare` in a Git repo with a dirty index | `git status --porcelain` shows only the expected new/modified `.tpatch` paths; the index checksum is unchanged |
| PIB-107 | S | the whole `prepare` call graph | every Git spawn routes through the one closed read-only G1–G4 allowlist; no unlisted argv or `gitutil` write entry point exists |
| PIB-108 | C | rename order | the observed rename sequence equals §7.2's order, with `status.json` last |
| PIB-109 | C | fsync sequence | each entry's parent directory is fsynced after its rename; the journal directory is fsynced after clear |
| PIB-110 | C | rename 2 of 4 fails; rollback succeeds | exit 5; every canonical file, `status.json` and the archive index byte-identical to the pre-run state; journal cleared; the message does **not** claim the whole tree is unchanged, and lists any orphan blob |
| PIB-111 | C | rename 2 of 4 fails and rollback also fails | exit 6; journal **retained**; report names the journal, the archive and the failing entry |
| PIB-112 | C | rollback attempted when a published entry no longer matches its new-image | rollback refuses that entry; exit 6; nothing overwritten |

### 18.9 H — Crash injection and recovery

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-113 | C | the mutating command run twice after a crash | the first recovers and exits 0 `recovered`; the second finds no journal, emits no `recovered` outcome and performs the requested work; the tree after the first run is identical to the tree recovery produced |
| PIB-114 | C | recovery with two `stage-*` trees for the slug | both removed; no blob removed; the index untouched |
| PIB-115 | C | recovery with a pending journal for another slug | that slug's lane is untouched |
| PIB-116 | C | crash phase CP0 (before lock) | next run proceeds normally |
| PIB-117 | C | crash phase CP1 (workspace directory flock held, no journal) | process death releases the descriptor; next run reopens the workspace root, acquires once, removes staging and proceeds |
| PIB-118 | C | crash phase CP2 (blobs, no journal) | blobs remain as reported orphans; the external locator stays outside cleanup; no index entry was added; advisory names purge |
| PIB-119 | C | crash phase CP3 (journal, no rename) | journal cleared; every canonical file byte-identical to pre-run |
| PIB-120 | C | crash phase CP4 (2 of 4 renamed) | the 2 published entries are restored to preimage; journal cleared |
| PIB-121 | C | crash phase CP5 (artifacts new, index old) | all restored; journal cleared |
| PIB-122 | C | crash phase CP6 (index new, status old) | all restored including `index.json`; journal cleared |
| PIB-123 | C | crash phase CP7 (everything new, journal not cleared) | recovery **undoes nothing**; journal cleared; tree stays all-new |
| PIB-124 | I | a second mutating `prepare` while a live process holds the workspace directory lock | exit 3, `transaction-in-progress`; the first run's outcome unaffected |
| PIB-125 | I | two mutating `prepare`/purge runs on different slugs in one workspace | exactly one holds the workspace directory authority; the other exits 3 `transaction-in-progress` |
| PIB-126 | C | crash phase CP9 (a third party rewrote a published entry) | exit 6; every file, the journal and the archive preserved and named |

### 18.10 I — Lifecycle, states and status metadata

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-127 | I | `prepare` on a `rejected` feature | exit 3, `state-refused`; remediation names `tpatch reopen <slug>`; zero mutation |
| PIB-128 | I | reopen, then `prepare` | allowed iff the restored `PriorState` is in the allowed set |
| PIB-129 | I | successful `prepare` on a feature with a `verify` record | `status.verify` is byte-identical afterwards |
| PIB-130 | I | `define --manual` on a zero-byte `spec.md` | exit 0; state `defined` — unchanged loose behavior |
| PIB-131 | I | `prepare --manual` on the same tree | exit 2 |
| PIB-132 | I | composite of PIB-130 then PIB-131 in one workspace | both observables in one run; zero mutation from the second |
| PIB-133 | I | `doctor` with a pending prepare journal | a `D9` warning naming the slug; exit code unchanged from the no-journal case |
| PIB-134 | I | `doctor` with a completely clean lane/archive state | D9 emits no finding at all, and makes no statement about whether an authority is held |
| PIB-135 | C | `doctor` with a pending journal, filesystem/authority spy | zero writes, zero `os.OpenRoot`/`Root.Open(".")` calls and zero `flock`/`SyscallConn.Control` calls |
| PIB-136 | G | `doctor` D1…D8 output vs pre-change goldens | byte-identical |
| PIB-137 | I | legacy `status.json` with no optional fields | after a successful `prepare`, only `state`/`last_command`/`updated_at`/`notes` differ; no field is added |
| PIB-138 | I | feature with no `artifacts/` directory | created 0755; no other feature's directory touched |
| PIB-139 | I | G1-established Git worktree with untracked `.tpatch/` | exit 0; advisory `bundle-untracked-in-git` present |

### 18.11 J — Provenance boundary

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-140 | I | `prepare --check` after any mutating `prepare` | every artifact still reports `provenance: "unknown"` |
| PIB-141 | S | the inspector's call graph | it never reads `intent-archive/**`; no import, no path constant |
| PIB-142 | G | every shipped string of both report schemas | no field, value or message asserts Path A vs Path B for a feature |
| PIB-143 | S | the shipped forbidden-inference list | it contains an `intent-archive` entry |
| PIB-144 | G | `generator` field | present only in the mutating report schema; the guard walks the declared keys of every wire struct this PRD writes and the §13.5 persistence sinks and asserts no `generator`-class key is declared or persisted; it performs **no** `.tpatch/` byte scan |
| PIB-145 | G | source + docs scan | ADR-034 is never cited as precedent for persistence; ADR-035 governs every write |
| PIB-146 | G | heuristic-mode run | the sidecar carries `heuristic_mode: true`, byte-compatible with `analyze`'s output for the same input |
| PIB-147 | G | the archive index and blob layout | carry no author, agent, model, provider, endpoint, or Path-A/Path-B field; sensitivity fixture proves the guard fails when one is added |

### 18.12 K — Security, privacy and forbidden fields

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-148 | U | publication target is a symlink at rename time | transaction aborts (exit 5); the symlink and its target are byte-identical afterwards |
| PIB-149 | U | publication target is a directory at rename time | exit 5; nothing written |
| PIB-150 | U | publication target is a FIFO at rename time | exit 5; nothing written; no open of it |
| PIB-151 | U | `artifacts/` is a symlink at publication time | exit 5; not followed |
| PIB-152 | I | `land` after a `--regenerate` | archive blobs and `index.json` are staged like any other `artifacts/**` file; no new refusal |
| PIB-153 | I | `record` after a `--regenerate` | `artifacts/post-apply.patch` is byte-identical to the same run without an archive present |
| PIB-154 | S | every write target constructor | derives from the compile-time name list plus the validated slug; no write path derives from provider output or file content |
| PIB-155 | G | every shipped message, help string and PRD/ADR sentence this cluster owns | no occurrence of "atomic", "atomically" or "simultaneously" applied to the multi-file publication; the guard fails when a fixture sentence adds one |
| PIB-156 | I | any refusal | the raw `<slug>` argument is never echoed when it fails canonical validation |
| PIB-157 | C | a provider response larger than the cap | V2 rejects it; exit 2; zero canonical mutations; the oversized bytes never reach a tracked path |
| PIB-158 | G | JSON report key walk at every nesting level | no key from the forbidden set (absolute path, timestamp, duration, hostname, pid, content, symlink target, secret, token, env value) |
| PIB-159 | G | sensitivity fixture for PIB-158 | injecting a `created_at` key makes the guard fail |

### 18.13 L — Determinism and canonical encoding

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-160 | U | two archive publications of identical prior bytes | byte-identical blob files and byte-identical index entries |
| PIB-161 | G | `index.json` bytes | no wall-clock field; key order fixed; `replaced` sorted by `artifact_id` |
| PIB-162 | U | `generation_id` derivation | one derivation point; equal canonical bodies produce equal ids; a one-byte change produces a different id |
| PIB-163 | I | success report | the `cp` restore form is printed verbatim and names no verb that does not exist |
| PIB-164 | G | journal bytes | no wall-clock field; no `phase` field; strict decode rejects an unknown field and trailing content |
| PIB-165 | S | every wire struct this PRD adds | no Go `map` type in any of them |
| PIB-166 | U | JSON report key order | equals the declared struct order in both the ordinary and refusal shapes |
| PIB-167 | I | two identical heuristic runs on identical trees | artifacts and reports byte-identical; only `status.json`'s `updated_at` differs |
| PIB-168 | U | archive index round-trip | decode→encode is byte-identical for a fixture with three generations |
| PIB-169 | G | blob filenames | match `^[0-9a-f]{64}\.blob$`; never uppercase hex, never truncated |

### 18.14 M — Output, exits and precedence

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-170 | I | default output | progress lines appear on **stderr**, never stdout |
| PIB-171 | I | `--json` and `--quiet` | no progress line appears on either stream |
| PIB-172 | I | both report schemas | `command` is `"prepare --check"` vs `"prepare"`; a consumer can switch on it |
| PIB-173 | I | refusal before inspection (bad slug) | `artifacts` is `[]`; `archive` omitted; `refusal.code` from the closed set |
| PIB-174 | I | refusal after inspection | `artifacts` carries the planned dispositions; `refusal.code` set |
| PIB-175 | G | advisory selection | a total function over observed state; no advisory can contradict its own artifact row; sensitivity fixture proves it fails when one does |
| PIB-176 | I | `prepare ../../etc --check`-less form outside a workspace | slug refusal (exit 3) precedes workspace discovery; the raw argument is not echoed |
| PIB-177 | C | pending journal plus a `rejected` feature | recovery runs and clears the journal, then the invocation **returns exit 0 `recovered`**; the exit-3 lifecycle refusal is not evaluated at all in that invocation, and appears only on the operator's retry |
| PIB-178 | I | every emission, both surfaces | the frozen disclaimer string byte-for-byte |
| PIB-179 | I | every refusal code | remediation names only shipped commands/flags/paths |
| PIB-180 | G | every refusal string | contains no `docs/`, no `.md`, no `http` |
| PIB-181 | S | the read path | no `os.Stat`, `os.Lstat`, `os.Open`, `os.ReadFile`, `filepath.Join(repoRoot, …)` |
| PIB-182 | S | the write path | targets come from the closed compile-time name list only |
| PIB-183 | U | pre-rename `Lstat` gate | a non-regular, non-absent target aborts before the rename |

### 18.15 N — Generator extraction and generation semantics

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-184 | S | the three extracted generators | none takes a `*store.Store` and none holds a writer |
| PIB-185 | C | the three extracted generators, filesystem spy | zero writes of any kind |
| PIB-186 | G | `analyze`/`define`/`explore` after the refactor | stdout, stderr, exit code and every written byte identical to pre-change goldens |
| PIB-187 | U | preserved artifact read as generation context | goes through the accepted bounded rooted capture; an unsafe one already refused at admissibility |
| PIB-188 | C | provider retry with `--no-retry` unset | no raw attempt/transcript sink, retry store, report/history/journal/archive field or file receives provider response/prompt text; intended canonical staged output remains the only allowed publication temporary |
| PIB-189 | S | the `prepare` generation path | `RetryOptions.Store` is nil at every construction site |
| PIB-190 | I | failed generation with retries | retained state contains only intended canonical staged outputs and allowed hashed/redacted metadata; the assertion is structural over sinks/paths/types and contains no retry/raw-attempt or transcript artifact (it does not content-scan canonical staged output, and remains sensitive: adding a transcript file makes it fail) |
| PIB-191 | G | shipped strings and docs | no claim that the command cannot hang or has bounded runtime |
| PIB-192 | C | provider deadline expiry during generation | heuristic fallback for that artifact; exit 0; advisory `provider-fallback-heuristic` |
| PIB-193 | C | deadline expiry after staging, before publication | the publication proceeds; exit 0 |
| PIB-194 | G | the validation set | exactly V1…V6; no check inspects headings, length or topicality |
| PIB-195 | I | successful `prepare`, then `prepare --check` in the same tree | `structural_readiness: ready`, exit 0 |
| PIB-196 | I | no provider configured, **default mode** | all missing artifacts generated heuristically; exit 0; advisory `provider-not-configured` |
| PIB-197 | I | provider configured but unreachable, **default mode** | heuristic fallback, exit 0 — **not** a command failure; `--regenerate` on the same tree refuses instead (PIB-369) |

### 18.16 O — `--check` compatibility

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-198 | G | `--check` human output vs pre-change goldens | byte-identical across the ready / not-ready / abort populations |
| PIB-199 | G | `--check --json` vs pre-change goldens | byte-identical |
| PIB-200 | I | `--check` exit codes | 0/2/3 unchanged for every accepted population |
| PIB-201 | C | `--check`, filesystem spy | zero writes |
| PIB-202 | I | `--check` | `provenance: "unknown"` for all four artifacts |
| PIB-203 | I | `--check` | the frozen disclaimer byte-for-byte |
| PIB-204 | I | `--check` abort populations | `artifacts` is `[]` iff `abort` is present |
| PIB-205 | I | `--check` | the accepted `overall` totals unchanged |
| PIB-206 | C | `--check` executed inside a publication window | reports what is on disk; a mid-rename artifact may be `unstable`; no crash, no lock, no write |
| PIB-207 | G | `--check` with and without a pending journal, identical canonical bytes | byte-identical output |

### 18.17 P — Non-invalidation of shipped commands

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-208 | G | `next` for all twelve states, incl. the `exploration.md` branch | byte-identical to pre-change goldens |
| PIB-209 | G | `cycle` end-to-end | byte-identical to pre-change goldens |
| PIB-210 | G | `analyze\|define\|explore\|implement` | byte-identical to pre-change goldens |
| PIB-211 | G | the same four with `--manual` | byte-identical to pre-change goldens |
| PIB-212 | G | `status`, `verify`, `record`, `land`, `reconcile` on a feature that never ran `prepare` | byte-identical to pre-change goldens |
| PIB-213 | S | reverse call graph | nothing imports or invokes the prepare publication package except the `prepare` command file; `prepare` is not an `OnComplete` in `next` and not a step in `cycle` |
| PIB-214 | S | `store.Store` writers | no new writer is added to `internal/store` beyond the archive functions named in S3 |

### 18.18 Q — Docs, skills and asset parity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-215 | G | `SPEC.md` | documents five modes, the complete flag mutex table, all seven exit-code rows including retired `4`, the `FEATURES.md` carve-out and the `feature intent-archive` grammar |
| PIB-216 | G | all six skill files | each names `tpatch prepare`; `requiredCommands` extended |
| PIB-217 | G | all six skill files | `prepare` appears in **no** phase-ordering table and **no** preflight block; sensitivity fixture proves the guard fails when it is added |
| PIB-218 | G | all six skill files | no sentence claims semantic certification or provenance |
| PIB-219 | G | `docs/feature-layout.md` | documents `intent-archive/`, its immutability, that it is never canonical truth, and the `cp` restore form |
| PIB-220 | G | `docs/agent-as-provider.md` | carries the `prepare --manual` row and the corrected `notes` sentence |

### 18.19 R — Platform and build

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-221 | G | `go vet` and target build matrix | clean for linux/amd64, linux/arm64 and darwin/arm64; Windows compiles the accepted read-only path but mutating prepare is source-guarded to refuse |
| PIB-222 | I | mutating prepare on Windows | exit 3 `prepare-unsupported-platform` before a held root authority or write; accepted Windows `--check` remains unchanged |
| PIB-223 | U | `freebsd` and another unsupported mutating `GOOS` | mutation refuses before workspace-root authority open; accepted read-only check keeps its own allowlist |

### 18.20 S — Totality, ledger and sensitivity guards

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-224 | G | §6.1 and §6.3 disposition tables | total over the accepted nine-value enum; adding a tenth value fails compilation or the guard |
| PIB-225 | G | §12.2 state table | total over `ValidFeatureState`; a thirteenth state fails the guard rather than defaulting to allowed |
| PIB-226 | G | the closed vocabularies of §10.2 | the shipped constant sets equal the tables exactly, including `outcome`'s `purge-partial` and `recovery-required`, the closed `pending_purge.selector` set and the one-value `execution_preflight` set |
| PIB-227 | G | the advisory catalog | exactly seventeen codes; every one reachable by a named fixture; a sensitivity fixture proves an eighteenth code fails the guard |
| PIB-228 | G | the refusal-code catalog | closed; every code reachable; every code has a remediation |
| PIB-229 | G | every `PIB-NNN` cited in prose | resolves to a real matrix row |
| PIB-230 | G | the acceptance ledger | every row maps to a resolvable package, `func TestX(*testing.T)` declaration and optional literal subtest, via AST — not a byte scan |
| PIB-231 | G | sensitivity meta-check | every row whose Kind contains `G` has a fixture proving the guard fails on a wrong input |
| PIB-232 | G | the injection seams of §18.1 | all are `nil` at init; no production path assigns one; a sensitivity fixture proves the guard fails when one is assigned |

### 18.21 T — Feature-request input

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-233 | I | `request.md` absent, empty, symlinked or unreadable, in `generate` and `regenerate` modes | exit 3, `request-unreadable`; zero mutation; a provider spy records zero calls |
| PIB-234 | I | `request.md` absent, `--manual` mode on a complete bundle | exit 0; adoption succeeds; generation is not required |

### 18.22 U — Flag deltas and grammar completeness

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-235 | I | `prepare <slug> --check --dry-run` | exit 1; mutual-exclusion text; whole `.tpatch/` tree byte-identical |
| PIB-236 | I | `prepare <slug> --manual` before/after fixture pair | before: exit 1 `unknown flag`; after: adoption path reached — the §5.3 row asserted as a pair, not as prose |
| PIB-237 | I | `prepare <slug> --regenerate` before/after pair | before: exit 1 `unknown flag`; after: the regenerate path is reached (provider gate may then refuse) |
| PIB-238 | I | `prepare <slug> --abandon-transaction` before/after pair | before: exit 1; after: preview printed, zero writes |
| PIB-239 | I | `prepare <slug> --allow-heuristic` alone (no `--regenerate`) | exit 0 with advisory `allow-heuristic-redundant` in default mode; exit 1 with `--check`/`--manual`/`--abandon-transaction` |
| PIB-240 | I | `prepare <slug> --dry-run` before/after pair, plus `--dry-run --abandon-transaction` | before: exit 1; after: `outcome: planned` for the three dry-run modes, and exit 1 with cobra mutual-exclusion text when combined with `--abandon-transaction` |
| PIB-241 | I | `prepare <slug> --timeout 5s` and `--timeout-phase 5s` | both accepted in `generate`/`regenerate`; both exit 1 with `--check`, `--manual`, `--abandon-transaction` |
| PIB-242 | I | `prepare <slug> --no-retry` | accepted in `generate`/`regenerate`; exit 1 in the other three modes |
| PIB-243 | I | `prepare <slug> --yes` without `--abandon-transaction` | exit 1; the fixed literal `prepare: --yes is only valid with --abandon-transaction` on stderr from the command's own preflight, **not** cobra mutual-exclusion text; zero writes |
| PIB-244 | G | the §5.3 flag delta table and the §5.1 grammar block vs the shipped flag set | all twelve registered flags are present in the registration set, each has exactly one delta row, each appears on at least one §5.1 grammar line in every mode where it is legal and on none where it is not, and every delta row names a registered flag; a sensitivity fixture adds a flag, and a second omits a legal flag from its mode's grammar line, and both make the guard fail |

### 18.23 V — Default-mode coherence and sidecar preservation

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-245 | I | `analysis.md` absent, `spec.md` present-nonempty | exit 2, `incoherent-bundle-gap`; zero mutation; nothing generated |
| PIB-246 | I | `analysis.md` absent, `exploration.md` present | exit 2, `incoherent-bundle-gap`; zero mutation |
| PIB-247 | I | `analysis.md` absent, `spec.md` and `exploration.md` present | exit 2, `incoherent-bundle-gap`; zero mutation |
| PIB-248 | I | `analysis.md` present, `spec.md` absent, `exploration.md` present | exit 2, `incoherent-bundle-gap`; zero mutation |
| PIB-249 | I | the `incoherent-bundle-gap` message | names `--regenerate` and `--manual` as the two routes; contains no `rm`, no `docs/`, no `.md` path |
| PIB-250 | G | the §6.1.2 coherence table | total over all eight present/absent combinations; a ninth synthetic combination fails the guard rather than defaulting to "generate" |
| PIB-251 | C | default mode, `analysis.md` preserved, sidecar `present-nonempty`, write spy | zero `OpenFile`-for-write and zero rename targeting `artifacts/analysis.json`; sidecar byte-identical |
| PIB-252 | I | default mode, `analysis.md` preserved, sidecar `present-empty` | sidecar still zero bytes afterwards; no refusal caused by it; exit 0 or `no-op` per the Markdown states |
| PIB-253 | I | `analysis.md` absent, sidecar `present-nonempty` | exit 2, `incoherent-bundle-gap`; sidecar byte-identical; no analysis generated |
| PIB-254 | I | `analysis.md` absent, sidecar `invalid-structured` | exit 2, `incoherent-bundle-gap`; sidecar byte-identical |
| PIB-255 | I | any successful default-mode run | no `artifacts/intent-archive/` directory exists afterwards |
| PIB-256 | C | any default-mode run, filesystem spy | zero writes under `artifacts/intent-archive/`; zero blob creations; the journal contains no `replace` entry |

### 18.24 W — `FEATURES.md`, status truth and the T1 carve-out

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-257 | C | `--manual` success, filesystem spy over the whole repo | exactly two write targets: `status.json` (via one rename) and `FEATURES.md`; the report's write-set statement matches the spy |
| PIB-258 | G | every shipped string and doc sentence this PRD owns | none claims `--manual` writes exactly one file, and none includes `FEATURES.md` in the publication unit |
| PIB-259 | C | publication rolled back after `status.json` was published | `status.json` is back to its preimage **and** a second `FEATURES.md` refresh ran; the index names the restored state |
| PIB-260 | C | recovery (next invocation) restores `status.json` | the same best-effort refresh runs as recovery's last act; failure of the refresh does not change the exit code |
| PIB-261 | I | `FEATURES.md` unwritable during a successful publication | exit 0; `status.json` correct; advisory `features-index-refresh-failed` present; no row and no message claims `FEATURES.md` is transactional |

### 18.25 X — Archive redaction gate and retention disclosure

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-262 | C | `--regenerate` where `spec.md` contains a `bearer-or-key-token` match, blob-write spy | exit 3, `archive-content-refused-sensitive`; **zero** blob files created; zero canonical mutations; the scan ran before the first blob write |
| PIB-263 | I | the same refusal's human and JSON output | names the artifact id and the matched class codes; contains no matched bytes, no excerpt, no offset, no line number |
| PIB-264 | U | one fixture per class (`private-key`, `connection-url`, `email-pii`, `credential-assignment`, `bearer-or-key-token`, `home-absolute-path`) | each refuses `--regenerate` with the same code and names its own class |
| PIB-265 | I | `--regenerate` after the operator removes the sensitive line | exit 0; the cleaned bytes are archived; the earlier refusal left nothing behind |
| PIB-266 | S | the archive write path | `redact.Scan` is called on in-memory bytes before any blob file is created; no code path writes a blob first and scans after |
| PIB-267 | G | shipped strings and docs | no flag, config key or message offers to skip or override the redaction scan; a sensitivity fixture adding a `--force-archive` string fails the guard |

### 18.26 Y — Dry-run with pending recovery, and the abandon route

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-268 | I | `--dry-run` with a pending journal, all three modes | exit 3, `recovery-pending`; no plan printed; the journal is read as a marker only and is never strict-decoded; journal/staging remain byte-identical and no directory authority is acquired |
| PIB-269 | I | the `recovery-pending` message | names the mutating re-run and `--abandon-transaction`; never claims the plan would be unchanged |
| PIB-270 | I | `--abandon-transaction --yes` on a divergent slug | whole `.tpatch/features/<slug>/` subtree byte-identical afterwards, including `status.json` and every blob |
| PIB-271 | I | the same run | journal, both preimage files and every `stage-*` tree now live under one `abandoned-<12hex>/`; nothing was deleted |
| PIB-272 | I | the abandon report | names the evidence directory, the `rm -rf` form, and states that no canonical file changed |
| PIB-273 | C | `--abandon-transaction` without `--yes`, filesystem spy | zero writes, zero renames, zero removals; exit 0; `outcome: abandon-planned` |
| PIB-274 | I | mutating `prepare` after a successful abandon, for each of the corrupt, divergent and recoverable evidence fixtures | no `recovery-pending`, no exit 6, no `recovered` outcome (nothing is pending after an abandon); the run proceeds on artifact evidence and reaches exit 0 on an admissible tree |

### 18.27 Z — Undo CAS and the metadata entries

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-275 | C | a third party rewrites a published entry, then rollback reaches it | the undo refuses that entry; exit 6; the third party's bytes are byte-identical afterwards |
| PIB-276 | C | a third party removes a published `create` entry before its undo | the undo refuses (identity is absence, not the new image); exit 6; nothing is recreated |
| PIB-277 | U | the journal of a `--regenerate` run | `archive_index` and `status` appear as full entries with `preimage`, `new_image` and `action`, exactly like artifact entries |
| PIB-278 | C | `status.json` modified by another process between its publication and a rollback | the undo refuses; exit 6; the other process's `status.json` survives |

### 18.28 AA — Lock authority, lane gate and platform envelope

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-279 | I | G1-established Git workspace, `.tpatch/local/` not covered by an ignore rule | exit 3, `local-lane-not-ignored`, before any staging byte; no local-lane mutation occurs |
| PIB-280 | I | Git-backed workspace with a tracked file under `.tpatch/local/` | exit 3, `local-lane-not-ignored`; zero mutation |
| PIB-281 | C | the lane gate, git-exec spy | the observed argv set is exactly {G1, G2, G3} for a non-`--regenerate` **normal mutating** run (`generate` or `manual`); the same spy over `--abandon-transaction`, `--dry-run`, `--check`, `intent-archive list` and `intent-archive purge` observes the empty set |
| PIB-282 | I | G1 plus both lane-gate halves pass, in a normal mutating mode | the run continues past the gate to the first local-lane write; the one directory authority was already acquired at §10.5 step 9, before the gate, and no cache/lock artifact is created |
| PIB-283 | S | the lane-gate call site | it runs before the first `.tpatch/local/` write in every **normal mutating** mode — `generate`, `manual` and `regenerate` — and is not reached at all from `--abandon-transaction`, `--dry-run`, `--check`, `intent-archive list` or `intent-archive purge`, none of which writes into that lane |
| PIB-284 | I | a process killed while holding the workspace directory flock | next run acquires the released directory authority; no stale-lock artifact/refusal exists |
| PIB-285 | S | the prepare authority implementation | no cache/user-cache/key/lock-file/named-semaphore path or HOME/XDG/LocalAppData read exists; authority is `Root.Open(".")` plus flock |
| PIB-286 | G | every `tpatch feature resource` subcommand beside the new prepare authority | stdout, stderr, exit code and written bytes byte-identical to pre-change goldens; prepare introduces no rescap extraction |
| PIB-287 | G | `rescap`'s unsupported-platform refusal after the extraction | byte-identical string and code; the `linux \|\| darwin` envelope is unchanged for resource capture |
| PIB-288 | U | mutating `prepare` on BSD, Windows or another unsupported target | exit 3, `prepare-unsupported-platform`, before workspace authority open and before any write |
| PIB-289 | U | `prepare --check` on the same BSD target | unchanged accepted behavior; ADR-034's `unix \|\| windows` allowlist still governs the read half |
| PIB-290 | I | two live mutating processes, same workspace and any slugs | the second contends on the workspace directory authority; the first completes normally |
| PIB-392 | U | Linux/Darwin locked-root filesystem classification | known NFS/CIFS/SMB/FUSE family is `lock-filesystem-unsupported`; overlayfs and representative exFAT/ZFS/f2fs/unknown classes are not rejected solely for class absence and still require real flock; `--check` unaffected |
| PIB-393 | G | docs/help authority guard | documents the locked-root-inode denylist, nested-mount/non-cross-machine limits and detected-class remediation without an absolute path; sensitivity fixture reintroducing an allowlist or all-write-target claim fails |
| PIB-394 | G | refusal/platform catalog | Linux/Darwin local-filesystem mutation versus Windows/BSD/other refusal is explicit; `prepare-unsupported-platform` and `lock-filesystem-unsupported` stay distinct and absent from `--check` |

### 18.29 AB — Crash liveness and per-entry CAS

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-291 | I | first process `SIGKILL`ed mid-transaction; second process runs | the second acquires the lock (the kernel released it), recovers, and reaches a terminal outcome; no refusal is left without the one route §10.4's partition assigns to its code |
| PIB-292 | I | `--manual` process `SIGKILL`ed after the lock, before the rename | the next `--manual` succeeds; no journal existed and none was needed |
| PIB-293 | I | crash at CP8 (journal cleared, lock not released) | the next run proceeds to exit 0 |
| PIB-294 | I | ten sequential kill-and-rerun cycles on one slug | every cycle terminates in a named outcome; every refusal names the one applicable route out — the mutating command, the abandon mode, or the §6.6/§9.7.2 manual procedure — and following it always yields progress; the final run publishes |
| PIB-295 | G | the CP table of §7.10 | every row's recovery outcome is reachable by a named injection fixture, and no row's outcome is "refuses forever" |
| PIB-296 | C | an editor writes `exploration.md` after set-level revalidation but before that entry's CAS | exit 5; entries already published are rolled back; the editor's bytes survive |
| PIB-297 | C | an editor writes entry 3 after entry 1 was renamed | rollback restores entry 1 to its preimage and refuses nothing; the editor's entry-3 bytes are untouched |

### 18.30 AC — Journal strict binding and identity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-298 | U | journal with trailing content after the JSON value (J1) | exit 6, `journal-corrupt`; nothing restored |
| PIB-299 | U | journal with an unknown field at any nesting level (J2) | exit 6, `journal-corrupt` |
| PIB-300 | U | journal with a different `version` (J3) | exit 6, `journal-version-mismatch`; the message says the journal was written by another build |
| PIB-301 | U | journal whose `slug` names a different feature (J4) | exit 6, `journal-foreign`; the other slug's lane untouched |
| PIB-302 | U | journal with a `rel` outside the feature directory (J7) | exit 6, `journal-path-escape`; no file outside the feature directory is read or written |
| PIB-303 | C | journal whose entry preimage was edited to point at attacker-chosen content (J8) | exit 6, `journal-forged`; the canonical file is byte-identical afterwards |
| PIB-304 | U | journal with a duplicate `artifact_id` or a `null` `entries` array (J9, J10) | exit 6, `journal-corrupt` |
| PIB-305 | G | the J1–J10 bind list | every bind is reachable by a named fixture; a sensitivity fixture removing one bind makes the guard fail |
| PIB-306 | U | identity comparison | equality requires all four of exists/sha256/size/mode; a same-size, same-mode, different-content file compares unequal |
| PIB-307 | C | a journal that passes J1–J10 but whose entry no longer matches any expected image | no write occurs; exit 6, `recovery-divergent` |

### 18.31 AD — Rooted publication writes and final verification

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-308 | S | every mutating prepare write path, tracked **and** local lane | no `os.Rename`, `os.CreateTemp`, `os.WriteFile`, `os.Remove`, `os.MkdirAll`, `filepath.Join(repoRoot, …)` or `gitutil.DurableWriteFile`; every write goes through a `*os.Root` method |
| PIB-309 | G | sensitivity fixture for PIB-308 | inserting one `os.Rename` call into a fixture package makes the scan fail |
| PIB-310 | C | ancestor redirect attempts an escape outside the held root | rooted write refuses; nothing is written outside the root; this row makes no in-root-redirection claim |
| PIB-311 | S | the publication path | `Root.Chmod`, `Root.Chown` and `Root.Chtimes` are never called |
| PIB-312 | G | `internal/store/store.go` and `internal/gitutil/index_snapshot.go` | `writeFileAtomicWithRename` and `DurableWriteFile` are unchanged and every shipped caller still reaches them; no `prepare` caller is added to either |
| PIB-313 | S | every mutating `prepare` write path, including manual, journal, preimages and staging | it never calls `writeFileAtomic`/`writeFileAtomicWithRename`/`SaveFeatureStatus`/`gitutil.DurableWriteFile` |
| PIB-314 | C | an external write lands between the last rename and the final verification | exit 6, `post-publication-divergence`; **nothing is undone**; the external bytes survive; the report names the entry |
| PIB-315 | I | an ordinary successful run | the final verification ran, and its result is what the exit-0 report asserts |

### 18.32 AE — Orphans, late crash phases, journal-loss boundary and concurrency

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-316 | C | a rolled-back `--regenerate` that had written two blobs | the exit-5 report lists both blob hashes and the exact purge command; the message does not claim the tree is byte-identical |
| PIB-317 | C | the same run | the number of listed orphans equals the number of blob files created by the attempt |
| PIB-318 | C | crash phase CP10 (`--manual`, during the single rename) | the next run finds no journal; `status.json` is old or new, never partial; exit 0 |
| PIB-319 | C | crash phase CP11 (retry after a blob write) | the existing blob is reused, not rewritten (inode and mtime unchanged); advisory `archive-blob-reused` |
| PIB-320 | C | recovery with an `abandoned-*` directory present | the directory is untouched; recovery removes only journal, preimages and `stage-*`; a subsequent abandon adds a second `abandoned-*` rather than merging or replacing the first |
| PIB-321 | I | `git clean -xfd` between a crash and the next run | next run has no journal and follows ordinary preflight; doctor does not claim it detected journal loss |
| PIB-322 | I | fresh clone after an interrupted committed archive state | no journal, no refusal; ordinary preflight; present blobs are not proof of journal loss |
| PIB-323 | G | doctor/docs disclosure | both state that a removed journal is unrecoverable **and ordinarily undetectable**; sensitivity fixture fails on a detection claim |
| PIB-324 | I | an external write to a published artifact after exit 0 | no tpatch postcondition is violated; the next `--check` reports the external content truthfully |
| PIB-325 | I | a concurrent `tpatch cycle` publishing `spec.md` mid-window | detected at the corresponding CAS; exit 5; `cycle`'s bytes survive |

### 18.33 AF — Git usage and per-mode archive semantics

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-326 | G | sensitivity fixture for the closed Git executor (PIB-107) | adding `git status` or any argv outside G1–G4 to a fixture path makes the allowlist/count guard fail |
| PIB-327 | I | G1 cleanly establishes a non-worktree | exit 0; advisory `workspace-not-git`; no G2–G4 invocation |
| PIB-328 | I | the same G1-established non-worktree under `--regenerate` | `bundle-untracked-in-git` is **not** emitted; `workspace-not-git` is |
| PIB-329 | I | G1 executable/semantic failure, including missing `git` | exit 3, `local-lane-unverifiable`; zero mutation; no `.git` existence heuristic is used |
| PIB-330 | I | `--regenerate` on a feature with all three artifacts absent | zero blobs, no index entry, no `intent-archive/` directory; exit 0 |

### 18.34 AG — Archive index strict decoding and tracking claims

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-331 | U | `index.json` with trailing content (X1) | exit 3, `archive-index-corrupt`; nothing written |
| PIB-332 | U | `index.json` with an unknown field (X2) | exit 3, `archive-index-corrupt` |
| PIB-333 | U | `index.json` with a higher `schema_version` (X3) | exit 3, `archive-index-version-unsupported`; the message says to upgrade tpatch and never says the file is broken |
| PIB-334 | U | `index.json` whose `feature` names another slug (X4) | exit 3, `archive-index-foreign` |
| PIB-335 | U | `index.json` with `generations: null` or `replaced: null` (X5) | exit 3, `archive-index-corrupt` |
| PIB-336 | U | duplicate or malformed `generation_id` (X6) | exit 3, `archive-index-corrupt` |
| PIB-337 | U | `blob: ""` with `purged: false`, and `blob` set with `purged: true` (X7) | both refuse with `archive-index-corrupt` |
| PIB-338 | U | a `path` escaping the feature directory (X8) | exit 3, `archive-index-path-escape`; no path outside the feature directory is touched |
| PIB-339 | U | an unknown `artifact_id` (X9) | exit 3, `archive-index-corrupt` |
| PIB-340 | U | a retained or tombstoned `generation_id` that does not reproduce from its immutable body (X10) | exit 3, `archive-index-generation-mismatch` |
| PIB-341 | G | sensitivity fixture for the strict decoder | a valid index plus one injected unknown key must fail; the guard is not satisfied by "decoding succeeded" |
| PIB-342 | G | shipped strings and docs | no sentence claims Git-independent or clone/clean-durable recovery for an uncommitted archive; `bundle-untracked-in-git`'s text states both risks |
| PIB-343 | I | regenerate → untracked archive → `git clean -fd` or `git clean -xfd` | clean can remove archive bytes; report/advisory state clone **and clean** loss risk, with no Git-independent durability claim |
| PIB-344 | G | the human report and `docs/feature-layout.md` | both disclose that `land` sweeps `intent-archive/**` into the operator's commit |

### 18.35 AH — The retention surface

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-345 | I | `feature intent-archive list <slug>` on a populated archive | every generation, entry, blob hash, size, presence flag and the `orphans:` section; exit 0 |
| PIB-346 | C | the same command, filesystem spy | zero writes of any kind |
| PIB-347 | I | `purge <slug>` with no scope selector | exit 1; the message names `--blob`, `--generation`, `--orphans`, `--all`; zero writes |
| PIB-348 | C | `purge <slug> --all` without `--yes`, filesystem spy | zero writes; the preview lists exactly what `--yes` would remove |
| PIB-349 | I | `purge` while a live prepare holds the workspace directory authority | exit 3, `transaction-in-progress`; whole-tree byte snapshot unchanged; it does not run pending-hash recovery without that authority |
| PIB-350 | I | `purge` with a pending journal for the slug | exit 3, `recovery-pending`; whole-tree byte snapshot unchanged; a decode spy records zero journal parses and zero journal renames; the message names the two recovery routes and neither is performed |
| PIB-351 | I | `purge --blob <h> --yes` run twice | the second is a no-op success; the index is byte-identical after the second run |
| PIB-352 | C | purge crash after a per-hash removal-pending index rename and before blob removal | index reports pending; retry under the authority removes the still-present hash-correct blob then tombstones it; no state claims absent bytes while the blob exists |
| PIB-353 | C | the same purge, crash injected before the index rewrite | nothing changed; the blob is still referenced and present |
| PIB-354 | I | `purge --generation <id> --yes` | every entry is tombstoned with `content_sha256` retained; generation id unchanged; strict decode passes |
| PIB-355 | I | `purge --all --yes` | every entry tombstoned; `blobs/` empty; `index.json` still present and still decodes |
| PIB-356 | G | `list`, `doctor` D9 and the purge report | all say "not recoverable until identical content is archived again", never report a hash as unavailable while a **retained or removal-pending** reference to it exists anywhere, and name only the one literal dangling repair of §9.7.3 for a dangling hash, the one literal repair §9.3.1 assigns to each tombstone-beside-blob subcase, §9.3.1's type-total removal-then-confirmed-purge or restore route for a present but unidentifiable blob under a retained reference, and the owning purge command for an **owned** hash; a sensitivity fixture that reports a live hash as absent, one that names `--orphans --yes` for a live blob, and one that offers a tombstone-beside-blob repair for a hash some reference to which is removal-pending, all fail the guard |
| PIB-357 | I | `purge --blob <h> --yes` where `<h>` is referenced by multiple generations | every reference to `<h>` is first pending, then tombstoned after one hash-correct blob removal; no reference is left live |
| PIB-358 | I | `purge --all --yes` on the same archive | every reference is selected, tombstoned and removed; no shared-reference refusal occurs |
| PIB-359 | I | `purge --orphans --yes` | captures/revalidates index preimage and each orphan regular-file identity/hash; removes exactly validated unreferenced blobs and rewrites no index |
| PIB-360 | I | the accidental-secret remediation path (`list` then `purge --blob`) | the blob file is gone; the index is tombstoned; exit 0 |
| PIB-361 | G | the purge report and `docs/feature-layout.md` | both state that a **committed** blob remains in Git history and that removing it from history is not something tpatch performs |

### 18.36 AI — Exit-6 escape, precedence and report shapes

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-362 | G | the nine journal/publication exit-6 codes of §10.4 | each message names the divergent entry, the file holding its prior bytes, and the `--abandon-transaction` command; `archive-purge-evidence-divergent` is **not** in this population and is asserted separately by PIB-506 |
| PIB-363 | I | each of the nine journal/publication exit-6 codes, then `--abandon-transaction --yes`, then a mutating re-run | the re-run reaches exit 0 on an admissible tree; no code in this population is permanent given its one applicable route (the archive population's equivalent is PIB-507) |
| PIB-364 | C | recovery with a live sibling process | the lock is acquired before recovery is attempted; recovery never runs while another process holds the lock |
| PIB-365 | C | `--regenerate` with no provider, provider and filesystem spies | zero provider calls, zero staged files, zero blobs; exit 3 `provider-required-for-regenerate` |
| PIB-366 | I | `--abandon-transaction --json` | `mode: "abandon"`, `artifacts: []`, `archive` absent, `abandoned` object present with the evidence directory |
| PIB-367 | I | `--quiet` across all five modes and all outcomes, plus `feature intent-archive purge` on each of its outcomes | every `prepare` line matches `prepare <slug>: <mode> <outcome>[ <detail>]` and every archive line matches `feature intent-archive purge <slug>: <outcome>[ <detail>]`; exactly one line in every case |

### 18.37 AJ — Provider authority, deadlines and leakage

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-368 | I | `--regenerate` with no provider configured | exit 3, `provider-required-for-regenerate`; remediation names `tpatch provider set`, `tpatch provider check` and `--allow-heuristic`; the bundle is byte-identical |
| PIB-369 | I | `--regenerate` with a provider that fails every attempt | exit 5, `regenerate-generation-failed`; the bundle byte-identical; staging retained; zero blobs |
| PIB-370 | C | `--regenerate`, per-phase deadline expires on the second artifact | exit 5; the message names which deadline expired; zero canonical mutations |
| PIB-371 | C | default mode, total deadline expires after the first artifact | the remaining artifacts are heuristic; advisories `provider-deadline-heuristic` **and** `provider-deadline-cascade`; exit 0 |
| PIB-372 | I | `--regenerate --allow-heuristic` with a failing provider | exit 0; advisory `regenerate-heuristic-allowed` plus the specific fallback advisory; prior bytes archived first |
| PIB-373 | G | the report of any run with ≥2 deadline-caused fallbacks | it states that one expiry caused them; no message presents them as independent provider failures |
| PIB-374 | I | `--allow-heuristic --help` text | contains the verbatim opt-in sentence of §11.3.2 |
| PIB-375 | S | config loading | no config key, env var or file setting can enable `--allow-heuristic` behavior; the flag is the only route |
| PIB-376 | G | every tpatch-managed prepare retry/lane/report/archive/journal construction | structural guards reject a retry store, raw-attempt/transcript sink or persistence path outside the canonical publication temporary; no provider-byte substring scan is asserted |
| PIB-377 | G | both report surfaces | no duration, deadline value or elapsed-time field appears in any output |

### 18.38 AK — Notes semantics, doctor evidence and residue reporting

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-378 | G | every shipped string, report field and doc sentence | none presents `status.json.notes` or the archive as evidence of who authored an artifact |
| PIB-379 | I | a `--regenerate` note read after a later `define` transition | the note is the later transition's, proving `notes` is last-transition state; no consumer treats it as history |
| PIB-380 | C | `doctor` with a live `prepare` running, authority spy | doctor creates nothing, opens no root and attempts no lock; it reports only durable evidence and makes no statement about the live authority; the running `prepare` completes with its outcome unaffected |
| PIB-381 | C | two concurrent `doctor` runs on one workspace, plus a third with an acquirable root | neither run observes or reports the other, because none opens or locks the root; no output claims a live authority or its absence |
| PIB-382 | I | `doctor` with retained `stage-*` trees | D9 names each path and the fact that the next successful run removes them |
| PIB-383 | I | `doctor` with an `abandoned-*` directory | D9 names it and the `rm -rf` form |
| PIB-384 | I | `doctor` with orphan blobs | D9 names the count and the exact `purge --orphans` command |
| PIB-385 | I | `doctor` with a corrupt `index.json` | D9 names the failing bind code and suggests only `list`; no repair is offered |
| PIB-386 | I | doctor with no durable lock artifact and nothing else | D9 is silent; no lock artifact, probe result or removal suggestion exists |
| PIB-387 | G | the §12.5 residue set | total over the observable durable lane and archive state; a synthetic additional residue class fails the guard rather than being ignored |

### 18.39 AL — Guards, skills and golden provenance

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-388 | G | sensitivity fixture for the redaction-message rule | a fixture message containing a matched substring makes the guard fail |
| PIB-389 | G | all six skill files | none presents the archive as a general history or undo facility |
| PIB-390 | G | all six skill files | each states that `--regenerate` requires a provider unless `--allow-heuristic` is passed |
| PIB-391 | G | the `--check` golden fixtures used by PIB-198 … PIB-207 | their provenance is the accepted `--check` implementation's commit range, not this cluster's; a fixture regenerated by this cluster's binary fails the guard |

### 18.40 AM — Rev-2 adjudication rows, amended by rev-3

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-395 | C | real holder and contender use the same held workspace-root directory inode | no lock/cache file exists; contender exits 3 while holder owns `Root.Open(".")` flock |
| PIB-396 | I | holder process is killed while holding the directory authority | next process acquires the released root-directory flock and reaches recovery; no persistent artifact exists |
| PIB-397 | I | two live real processes, same workspace but different slugs | exactly one owns the directory authority; the other exits 3 and no lock file exists |
| PIB-398 | G | directory-authority construction | exactly one `os.OpenRoot` and retained `Root.Open(".")` `*os.File` acquisition per mutating invocation; flock/unlock use `SyscallConn.Control`, helpers receive the authority rather than an fd, and no cache/key/HOME/XDG/LocalAppData/lock-path primitive exists |
| PIB-399 | C | ready `--manual` publication with rooted-operation spy | a same-directory rooted temp is fsynced then renamed to `status.json`; no `writeFileAtomic`, `SaveFeatureStatus`, or path-based writer is called; no journal/archive is created |
| PIB-400 | C | external `status.json` edit after manual preimage capture and before rename | exit 5 `status-changed`; the external status bytes and `FEATURES.md` are byte-identical; no rename occurs |
| PIB-401 | U | tombstoned index entry with immutable `content_sha256` | X10 recomputes the recorded generation id after purge; missing/mismatched digest, blob/purged inconsistency, or altered immutable body refuses |
| PIB-402 | I | later regeneration computes an existing fully tombstoned generation whose hash also appears elsewhere | redaction passes; blobs are written/reused before one CAS index rewrite rehydrates **every** tombstoned or removal-pending reference to that hash, no duplicate generation is appended, and no false tombstone/orphan remains |
| PIB-403 | I | repeated purge→rehydrate with a blob shared by two generations | ids/order remain stable; rehydrating hash h revives **every** tombstoned or pending reference to h, and later purge counts every current live reference |
| PIB-404 | C | rehydration redaction and crash-order subtests | a redaction match leaves tombstones/no blob; a crash before index rename leaves at most a blob beside its tombstone — CP13's residue, which X11 reports as exit-3 `archive-index-storage-inconsistent` and never as `archive-purge-evidence-divergent`, naming `--orphans --yes` when the hash is globally unreferenced and the confirmed `--blob <h> --yes` when another reference still holds it retained or removal-pending — while no index ever references a missing blob |
| PIB-405 | C | concurrent index edit after purge preimage capture and immediately before rooted rename | strict re-read/CAS refuses `archive-purge-index-changed` before **any** selected blob removal; live edited index and blobs survive |
| PIB-406 | C | ancestor becomes a relative symlink to another directory inside the held root | outside-root escape remains refused; an identity-changing in-root redirect is caught by CAS; fixture/docs do not claim detection of an equal-identity alias |
| PIB-407 | I | ordinary partial bundle made by `analyze`/`define`, with completely clean lane/archive | D9 emits neither residue nor journal-loss diagnosis; no recovery/repair is suggested |
| PIB-408 | I | linked worktree and submodule/nested workspace with no `.git` at the `.tpatch` root | G1 from the workspace root establishes `true`, runs the Git gate; Git exec failure remains `local-lane-unverifiable`, while Git-established non-worktree proceeds |
| PIB-409 | G | mutating platform/build matrix | mutation is Linux/Darwin-only and requires non-denied root-inode classification plus real flock; Windows, BSD and all other targets refuse; accepted read-only `unix \|\| windows` behavior is unchanged |

### 18.41 AN — Rev-3 adjudication: directory authority, privacy and archive truth

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-410 | C | mutating invocation with rooted-operation spy | exactly one held `os.OpenRoot` plus strongly retained `Root.Open(".")` directory `*os.File` supplies flock and all rooted I/O; no second authority root or naked-fd authority exists |
| PIB-411 | C | flock returns `EWOULDBLOCK`, `EAGAIN`, then another error | first two are only contention; another error is exit 3 `directory-flock-unavailable`, zero mutation and no unlocked fallback |
| PIB-412 | I | nested mutator/self-reentry and a different slug under one holder | held authority is threaded, never recursively acquired; every workspace mutator serializes across slugs |
| PIB-413 | I | real-process alias path acquired before rename, then workspace root rename while holder lives | the pre-existing alias contender identifies/contends on the held inode; the original discovered path becoming missing/different refuses rather than claiming that the renamed root was rediscovered |
| PIB-414 | C | root deleted/recreated before set-level publication check | exit 5 `workspace-root-changed` before the window; no exit 0 and replacement root bytes survive |
| PIB-415 | C | root deleted/recreated after a publication rename before final verification | exit 6 `workspace-root-replaced-after-publication`; evidence preserved, never exit 0 |
| PIB-416 | U | Linux/Darwin root-inode classification table including unknown/NFS/SMB/FUSE/overlay | known remote/user-space families deny with detected-class remediation; overlay and unknown/local examples proceed to the required real flock; no fixture claims nested mounts are classified |
| PIB-417 | U | Windows and BSD mutating builds | exit 3 `prepare-unsupported-platform`; accepted `--check` platform behavior is byte-identical |
| PIB-418 | G | authority source/docs sensitivity | reintroducing cache/key/lock-file/HOME/XDG/LocalAppData/named semaphore or claiming a rescap file-lock extraction fails |
| PIB-419 | C | provider success/failure/retry with structural staging/report spies | no retry store, raw-attempt/transcript sink or persistence path exists outside the allowed canonical staged output; final output may equal provider bytes |
| PIB-420 | G | pure generator/retry construction sensitivity | `RetryOptions.Store` is nil, no retry/raw-attempt/transcript sink exists, and only allowed ordinal/reason/hash/size metadata may persist |
| PIB-421 | C | external index/blob edit before purge immediate pre-CAS checks | `archive-purge-index-changed` or identity refusal occurs before removal; edited bytes survive |
| PIB-422 | C | external replacement in purge CAS→rename/remove final syscall window | documentation/report test states bytes can be overwritten/removed; no total-CAS/no-clobber claim passes |
| PIB-423 | I | `purge --blob h --yes` with h referenced by several generations | every `h` reference is pending in one CAS publication, its hash-correct blob is removed, then every pending `h` reference is tombstoned; no live reference remains |
| PIB-424 | I | `purge --generation id` shares h with an unselected generation | exit 3 `archive-blob-shared`, names h and exact `--blob h`/`--all` escalation; no mutation |
| PIB-425 | C | rehydrate h with tombstones in multiple generations | one CAS rewrite un-tombstones every tombstoned/pending h reference; a dangling **retained** h reference is not in the rehydrated set and still refuses; no tombstone says absent while h.blob exists |
| PIB-426 | I | regenerate → untracked archive → `git clean -fd` and `git clean -xfd` | either clean may remove `.tpatch/features/**`; no shipped recovery claim contradicts clone/clean loss |
| PIB-427 | S | G1–G4 exec seam with foreign `GIT_DIR` and call-count spy | every allowed invocation applies the §7.13 pinned scrub and C locale; G1 exactly once, G2/G3 consume GitState and relative lanes without rediscovery; dry-run/list spawn zero Git |
| PIB-428 | I | retained index reference whose blob is absent, non-regular or hash-mismatched | X11 reports a missing retained blob as repairable `archive-blob-dangling`; list, doctor and the refusal all name the same one literal confirmed-purge command and nothing else. A **present** but non-regular or hash-wrong blob under that retained reference is instead `archive-blob-corrupt` (exit 3, zero-write), rendered `corrupt` by `list`, and carries §9.3.1's type-total removal-then-confirmed-purge or restore route and no other; neither observation is ever reported as an orphan, as divergence or as dangling-when-present. Unsafe/wrong evidence under a **pending** reference keeps its own evidence-preserving refusal (PIB-543) |
| PIB-429 | C | `purge --orphans` after external index adds a reference | strict preimage/reference revalidation prevents removing the newly referenced blob |
| PIB-430 | C | external blob replacement immediately after orphan/selected revalidation | final-syscall residual is disclosed; test does not claim remove is atomic or preserves replacement |
| PIB-431 | G | refusal catalog, precedence, human and JSON renderers | every reachable code has one exit and identical code/remediation across catalog/human/JSON; purge/publication index codes stay distinct |
| PIB-432 | G | implementation-slice and rescap parity guard | S1b is prepare-owned directory authority, not a file-lock extraction; `rescap` files/behavior remain unchanged |

### 18.42 AO — Rev-4 adjudication: lifetime, liveness and protocol parity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-433 | C | real holder retains authority while forced GC runs and a real contender starts | contender remains exit 3 until the holder explicitly releases; GC never releases a live directory authority |
| PIB-434 | G | authority-lifetime sensitivity fixture | dropping the strong directory `*os.File` reference (or introducing a finalizer release) makes the lifetime guard fail |
| PIB-435 | C | nested recovery/publication/purge helpers under one holder | exactly one authority acquisition occurs; helpers receive the authority and never reacquire or retain only an fd |
| PIB-436 | I | real root rename after acquisition, before and after the publication window | the original discovered path missing/different gives exit 5 before the window or exit 6 after it; no report claims moved-path rediscovery |
| PIB-437 | G | command/lock-duration documentation guard | rejects a total-command or authority-hold bound while preserving the provider-only timeout contract |
| PIB-438 | G | central Git executor/refactor guard | rejects an unlisted Git argv, a second G1 probe, an absolute lane argument or a duplicated prepare privacy gate; legacy wrappers/goldens remain reachable |
| PIB-439 | I | Git process-count table with worktree/non-worktree/regenerate cases | worktree non-regenerate is G1+G2+G3, worktree regenerate is G1–G4, established non-worktree is G1 only; argv, scrub and C locale match §7.13 |
| PIB-440 | C | dry-run in every admissible/refusal path, process/authority/filesystem spies | it returns before the mutating platform/filesystem gate, G1–G4, lane gate, lock and recovery; zero Git spawn, flock or write |
| PIB-441 | U | root-filesystem policy fixtures | Linux NFS/CIFS/SMB/FUSE and Darwin NFS/SMB/WebDAV/macFUSE/osxfuse deny; overlayfs and unknown/local classes are not denied merely by absence and still require flock |
| PIB-442 | I | typical Linux and macOS runner two-process root-directory contention/release | one process holds the real flock, the peer contends, and explicit release lets the peer acquire; fixture scopes the result to the root inode |
| PIB-443 | C | crash injection before removal, after removal and after tombstone for each lexically ordered selected hash | recovery under the authority performs the one next action from pending evidence; `--all`/generation selection remains valid and no X11 state bricks |
| PIB-444 | I | a missing retained blob, with exact replacement bytes available on disk | ordinary regenerate and archive mutation still refuse — availability of the bytes changes nothing, because no reachable path consumes them; the confirmed purge is the only repair, and after it an ordinary regenerate rehydrates through §9.3 |
| PIB-445 | G | doctor source and output surface | no D9 code path opens the workspace root or calls `flock`; no D9 string reports, denies or speculates about a live authority; D1–D8 goldens stay byte-identical; a sensitivity fixture reintroducing a probe fails the guard |
| PIB-446 | G | raw-response structural sensitivity fixture | adding a retry store, raw-attempt/transcript sink/path or report/history persistence fails; a canonical staged output equal to provider bytes remains permitted |
| PIB-447 | G | §9.3 §9.7.1, §9.7.2, X11, CP12/CP12a/CP13 and ADR D10/D16 semantic parity fixture | all express pending → claim-globally → remove-or-finalize → tombstone-every-reference, deterministic recovery **performed only by `intent-archive purge --yes`**, the global pending-ownership invariant (a blob is removed only when no reference to its hash is anything but removal-pending), the single confirmed-purge dangling repair, §9.3.1's route for a present but unidentifiable blob under a retained reference, the globally-evaluated split of a tombstone beside a present blob into the `--orphans --yes` repair when its hash is unreferenced and the confirmed `--blob <h> --yes` repair when its hash is live-but-not-owned, the selector-independent whole-index X11 scan, and the `archive-purge-partial` boundary; a contradictory “pending must still exist”, “rehydration repairs a dangling reference”, “a mutating `prepare` finalizes a pending hash”, “a tombstone beside a present blob is purge divergence”, “a tombstone beside a present blob is always an orphan”, “the recovery removes the blob and leaves the other reference dangling” or “X11 validates only the selected references” sentence fails |
| PIB-448 | G | refusal/precedence/help/JSON/row catalog guard | root-path change, dangling repair and `archive-purge-partial` each have one exit and one remediation everywhere; archive list stays read-only/no-lock and no stale section reference remains |

### 18.43 AP — Rev-5 adjudication: reachability, rooted control writes and bounded totality

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-449 | I | `--abandon-transaction --yes` against a journal that fails a J1–J10 bind (corrupt, forged, version-mismatched and foreign fixtures) | exit 0 in every case; the evidence is moved, not decoded; no `journal-*` refusal is produced; a subsequent admissible mutating run reaches exit 0 |
| PIB-450 | C | `--abandon-transaction --yes` on CP9-shaped divergent evidence | exit 0; the whole `.tpatch/features/` subtree is byte-identical; nothing is restored, undone or removed; the report names the evidence directory |
| PIB-451 | C | `--abandon-transaction --yes` on a cleanly recoverable CP4 journal | exit 0; the two published entries are **not** undone; the journal is moved rather than consumed; the report states that no canonical file changed and does not imply a repair |
| PIB-452 | C | abandon-ordering spy over all of the above | the mutating platform/filesystem gate and the flock both ran, the lane gate did **not**, and neither journal recovery nor pending archive-hash recovery was invoked at all; the branch is taken at §7.8 step 2 / §10.5 step 10 |
| PIB-453 | I | `--abandon-transaction` on a slug with a removal-pending archive index and no journal, staging or abandoned evidence | exit 3, `no-pending-transaction`; `index.json` and every blob byte-identical; the remediation names the literal `feature intent-archive purge <slug> --blob <hash> --yes` archive route (or `--all --yes` when more than one hash is pending) and names **no** mutating `prepare` route, because `prepare` refuses `recovery-pending` in that state |
| PIB-454 | S | every mutating prepare write path, including journal, both preimages, staging, abandon moves and the derived-index refresh | each goes through the §7.7.1 rooted control writer; `gitutil.DurableWriteFile`, `writeFileAtomic*` and `os.CreateTemp` appear nowhere in them |
| PIB-455 | C | journal write with a rooted-operation spy | the observed sequence is same-directory `Root.OpenFile(O_CREATE\|O_EXCL)` → write → `Sync` → `Close` → `Root.Rename` → rooted parent-directory `Sync`, with no path-based call between them |
| PIB-456 | G | sensitivity fixture for PIB-454 | inserting a `gitutil.DurableWriteFile` call — or any path-based writer — into the local lane makes the scan fail |
| PIB-457 | I | dangling retained reference, then an ordinary `--regenerate` and an ordinary archive mutation | both refuse `archive-blob-dangling`; the message, `list` and doctor each name the identical literal command `tpatch feature intent-archive purge <slug> --blob <hash> --yes` and no alternative repair |
| PIB-458 | C | confirmed `purge --blob <h> --yes` on that dangling `h`, with a removal spy | every reference to `h` in every generation becomes a tombstone; the spy records **zero** removal attempts for `h`; exit 0; the index strict-decodes afterwards |
| PIB-459 | G | shipped strings, help text, reports and this cluster's docs | none offers rehydration, restoration or any second route as a dangling-reference repair; a sensitivity fixture reintroducing one fails the guard |
| PIB-460 | I | after PIB-458, an ordinary `--regenerate` that reproduces the tombstoned generation | the existing §9.3 global rehydration path runs: blobs are written/reused and one CAS rewrite un-tombstones every reference to `h`; no duplicate generation is appended |
| PIB-461 | C | `--dry-run` in every mode with process, authority and filesystem spies, over one admissible and one refusing fixture per evaluated class | every §10.5 step-7 sub-check executes inside the branch before return; zero mutating gate, zero Git process, zero flock, zero provider call, zero write |
| PIB-462 | I | any `--dry-run` report, human and JSON | carries `execution_preflight: "not_evaluated"` and the verbatim §6.4 sentence, including the clause naming platform, filesystem, Git, lock and recovery as still able to refuse |
| PIB-463 | I | `--dry-run` on Windows, on a denied root filesystem, and where G1 cannot classify Git | a plan (or an evaluated refusal) is printed in each case; `prepare-unsupported-platform`, `lock-filesystem-unsupported` and `local-lane-unverifiable` are never emitted; the not-evaluated field is present |
| PIB-464 | G | the §6.4 reproduced/non-evaluated tables against the §10.4.1 catalog | every catalog code appears in exactly one column; adding a code to the catalog without placing it fails the guard, and a code placed in both columns fails it |
| PIB-465 | C | one fixture per §9.7.2 preflight row (selector, strict decode, whole-index X11 storage, shared reference, orphan identity, reference count) | each exits 3 with a whole-tree byte snapshot identical to the pre-run tree; no pending record, tombstone or blob removal occurred. The X11 fixture is run with a selector that is **disjoint** from the inconsistent hash as well as with one that is not, and both refuse identically (PIB-542) |
| PIB-466 | C | I/O failure injected **inside** one hash's machine, after its removal-pending CAS (`failPurgeAfterFirstMutation`) | exit **5**, `archive-purge-partial`, never 3; the report carries `completed_hashes`, `pending_hash`, `remaining_hashes`, `resume: "pending-recovery-then-completion"`, the sanitized equivalent `retry`, `retry_cwd: "workspace-root"` and the consistent-state line; the index strict-decodes |
| PIB-467 | I | re-running the reported retry command from the workspace root after PIB-466 (branch (a) only) | the purge resumes from the pending hash — reporting exit 0 `recovered` for the pending hash, then exit 0 for the completed work list on the retry it names — and the index strict-decodes at every intermediate observation point |
| PIB-468 | C | divergent or unsafe purge evidence discovered after the first mutation | exit **6**, `archive-purge-evidence-divergent`, not 5; every blob, the index and the pending record are preserved and named |
| PIB-469 | G | every exit-3 population in the §10.4.1 catalog | each is asserted with a whole-tree byte snapshot, with **no** post-recovery qualification; two sensitivity fixtures fail the guard — a code path that writes and then returns exit 3, and one that recovers and then falls through to a later exit-3 gate |
| PIB-470 | C | `doctor` run against a workspace with a live mutating holder, syscall spy | D9 performs zero `os.OpenRoot`, zero `Root.Open(".")`, zero `flock` and zero `SyscallConn.Control` calls; the live holder's exit code and output are unaffected |
| PIB-471 | G | every shipped string, help text and doc sentence this cluster owns | none claims a diagnostic can identify a lock holder or prove no holder exists; `transaction-in-progress` states authority-held, holder-unknowable and wait/retry; a sensitivity fixture claiming holder identity fails |
| PIB-472 | S | the Git call graph after the S4 refactor | `internal/rescap/gitgate.go` routes its spawns through the one central executor; no second prepare privacy gate and no unrouted `exec.Command("git", …)` remains in the authorized files |
| PIB-473 | G | every existing caller of the ignore/tracked helpers, against pre-change goldens | each keeps its current environment, argv shape, exit-code interpretation and output through its explicit compatibility wrapper; a sensitivity fixture in which a wrapper alters its caller's environment or output fails the guard |
| PIB-474 | C | prepare's own G1–G4 invocations, environment spy | the child environment contains none of the §7.13 pinned variables, including `GIT_CONFIG_COUNT` and every variable matching `^GIT_CONFIG_(KEY\|VALUE)_[0-9]+$` regardless of the declared count; `LC_ALL=C` and `LANG=C` are set |
| PIB-475 | I | workspace whose `.tpatch/local/` is ignored only by the operator's global excludes file | the lane gate passes and the run proceeds; `local-lane-not-ignored` is not emitted; the spy shows `GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM`/`HOME` were preserved |
| PIB-476 | G | every G1–G4 argv element and every report/refusal/advisory field | none is an absolute path; a sensitivity fixture passing an absolute lane argument fails the guard |
| PIB-477 | G | the schema-scoped provenance guard | a fixture whose canonical artifact prose contains the word "generator" **passes**; a fixture adding a `generator`-class key to the report, `status.json`, the sidecar, `index.json` or the journal **fails** |
| PIB-478 | U | root classification seam | classification calls `fstatfs` on the already-held root-directory descriptor, after the root and `"."` are open and before the flock attempt; a fixture proves the classified object is the same inode that is locked, with no path-based `statfs` on the write path |
| PIB-479 | U | denied-class tables | Linux denies exactly `0x6969`, `0x517B`, `0xFF534D42`, `0xFE534D42`, `0x65735546`; Darwin denies exactly `nfs`, `smbfs`, `webdav`, `macfuse`, `osxfuse` by exact string equality; a third-party FUSE name such as `sshfs` is **not** denied by any prefix/suffix rule and takes the unknown-local route |
| PIB-480 | I | overlayfs and an unknown-but-local class | both proceed only after a real successful flock; docs and help disclose that an unrecognized local filesystem may accept flock without excluding anything, and that no cross-machine guarantee follows; a sensitivity fixture claiming cross-machine exclusion fails |
| PIB-481 | C | authority release followed by a `SyscallConn.Control` attempt, and a concurrent-close attempt from a second goroutine | the result is deterministic and is reported as neither an acquisition nor a lock loss; it fails closed as `directory-flock-unavailable` where reachable; the single-goroutine ownership rule of §7.4.1 is asserted by the fixture |
| PIB-482 | G | reference-truth guard over this PRD and ADR-035 | no sentence outside a clearly historical revision-history entry claims the prepare lock is extracted from `rescap`; every `§` reference, `PIB-NNN` reference and `Dn` reference resolves; a sensitivity fixture reintroducing an extraction claim or a dangling section reference fails |

### 18.44 AQ — Rev-6 adjudication: terminal recovery, reachability and flag grammar

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-483 | I | mutating `prepare` (each of `generate`, `manual`, `regenerate`) on a CP3–CP7 pending journal | exit 0; `outcome: "recovered"`; `action: "none"`; the `recovery` object names `kind: "journal-undo"` and every restored repo-relative entry; advisory `recovered-prior-transaction`; the requested operation is **not** performed and no artifact it would have written exists |
| PIB-484 | C | the same runs with an `afterRecoveryComplete` ordering spy | the command returns immediately after recovery: zero lifecycle-gate evaluations, zero artifact-admissibility checks, zero coherence checks, zero provider calls, zero staging writes and zero archive mutations occur after the recovery completes |
| PIB-485 | I | re-running the reported `retry` from the workspace root after PIB-483 | the originally requested operation is performed and exits 0 `published` (or the admissible refusal the tree actually warrants); no `recovered` outcome is emitted the second time |
| PIB-486 | I | `intent-archive purge … --yes` — and **only** that command — with removal-pending archive hashes and no journal | exit 0; `outcome: "recovered"`; `recovery.kind: "archive-purge-finalize"`; `finalized_hashes` lists exactly the finalized hashes; no new blob, no new tombstone beyond the pending set, and no selector processing occurred in that invocation; the same state under a mutating `prepare` yields exit 3 `recovery-pending` with zero writes instead (PIB-525) |
| PIB-487 | G | exit-3 zero-write guard, post-recovery arm | no reachable path reaches an exit-3 return after a recovery write in the same invocation; two sensitivity fixtures fail the guard — one that recovers and then evaluates the lifecycle gate, and one that recovers and then evaluates artifact admissibility |
| PIB-488 | C | recovery failure fixtures: CP9 divergence, each J1–J10 bind failure, and a refused undo CAS | exit **6** in every case with `outcome: "recovery-refused"`; never exit 0, never `recovered` and never the in-command `rolled-back` outcome of §7.9; every file, the journal and the archive are preserved and named |
| PIB-489 | I | `intent-archive purge <slug> --blob <h> --yes` with a pending prepare journal | exit 3, `recovery-pending`; whole-tree byte snapshot identical; a decode spy records zero strict decodes of the journal, zero journal renames and zero journal removals; zero canonical or index writes |
| PIB-490 | I | the same selection without `--yes` (preview) | identical refusal, exit 3 `recovery-pending`, zero writes and **zero** flock acquisitions; the preview plan is not printed |
| PIB-491 | I | `intent-archive purge --blob <h> --yes` with removal-pending hashes and **no** journal | exit 0, `outcome: "recovered"`, the pending hashes finalized; the selector is not processed in that invocation; re-running the reported sanitized retry then performs the purge and exits 0 |
| PIB-492 | I | `--abandon-transaction --yes` with `git` absent from `PATH`, and again with `git` present but unusable, over a corrupt-journal fixture | exit 0 in both cases; the evidence is moved; `local-lane-unverifiable` is never emitted; a process spy records **zero** Git spawns |
| PIB-493 | I | `--abandon-transaction --yes` in a worktree whose `.tpatch/local/` is **not** ignored | exit 0; the evidence is moved; `local-lane-not-ignored` is never emitted; the lane's ignore status is unchanged afterwards |
| PIB-494 | C | abandon-mode gate-ordering spy | the mutating platform/filesystem gate and the single flock both ran before the branch; G1–G4 did not run at all; neither recovery pass was invoked; the branch is at §7.8 step 2 / §10.5 step 10 |
| PIB-495 | I | `--abandon-transaction` refused by each of rows 5, 6 and 8 of §6.6's gate table — `prepare-unsupported-platform`, `lock-filesystem-unsupported`, `directory-flock-unavailable` — with journal evidence present | exit 3; the message names the repo-relative `.tpatch/local/intent-prepare/<slug>/` lane and the `rm -rf` last-resort procedure, states that it discards undo evidence and touches nothing under `.tpatch/features/`, and contains no absolute path; with an **empty** lane the same three refusals omit the procedure entirely; row 7 (`transaction-in-progress`) omits it in **both** cases (PIB-512) |
| PIB-496 | G | every shipped string, help text and doc sentence this cluster owns | no sentence claims an unqualified "always recoverable" or "exit 6 is never terminal" without the environmental qualification of §6.6/§10.4; a semantic sensitivity fixture asserting unconditional recoverability fails the guard |
| PIB-497 | G | every retry, remediation and report field, run with an absolute `--path <dir>` | the absolute pathname appears nowhere in stdout, stderr or the JSON report on the recovered, partial-purge, refusal and success paths; a sensitivity fixture that renders it inside a prose sentence rather than a field still fails the guard |
| PIB-498 | G | every emitter of a `tpatch` rerun this cluster ships, **derived** from the report model and the human renderers rather than from a list | the guard enumerates every site that hands the operator a `tpatch` command — pending-purge preview, terminal journal recovery, terminal pending-purge recovery, partial-purge resumption on all three `resume` branches, the archive-divergence rerun, the dangling-reference repair, §9.3.1's corrupt-blob repair and both §9.3.1 tombstone-beside-blob repairs — and asserts for each that `retry` is the equivalent command with no root-selection argv, that `retry_cwd` is the literal `workspace-root`, and that the human rendering carries the sentence `Run this again from the same workspace root:` **byte-for-byte at column 0**, with no leading or trailing whitespace, alone on its line, immediately above the command with nothing between them. Exactly one heading string exists in the shipped set. The three external shell forms §10.7 permits are excluded by construction, not by allowlist. Semantic sensitivity fixtures: one emitter using rev-8's `Run this again from the same workspace root to perform it:`; one using a numbered `3. run this again from the same workspace root:` step; one emitter with a correct heading but a blank line between it and the command; one that indents the correct heading by two spaces; one that appends a trailing space to it |
| PIB-499 | I | a second `--abandon-transaction --yes` when only `abandoned-<12hex>/` residue remains | exit 3, `no-pending-transaction`; the residue is byte-identical and not nested, moved or removed; the report lists each residue directory repo-relative and offers the optional `rm -rf` form; JSON carries `abandoned.existing[]` with exactly those paths |
| PIB-500 | I | `prepare <slug> --allow-heuristic` in the default mode | exit 0 (or the tree's admissible outcome); never exit 1; advisory `allow-heuristic-redundant` is present and the produced bytes are identical to the same run without the flag |
| PIB-501 | I | `--allow-heuristic` with each of `--check`, `--manual` and `--abandon-transaction` | exit 1; cobra mutual-exclusion text; zero writes |
| PIB-502 | I | `--yes` in every non-abandon mode — bare default, `--check`, `--manual`, `--regenerate` | exit 1 in all four; the identical fixed literal `prepare: --yes is only valid with --abandon-transaction` on stderr; zero writes; and, in the same table, bare `--abandon-transaction` previews at exit 0 while `--abandon-transaction --yes` acts |
| PIB-503 | G | the `--yes` validation source and strings | the fixed literal is declared exactly once and is the only text any illegal `--yes` combination produces; no `MarkFlagsMutuallyExclusive` call names `--yes` on `prepare`; a sensitivity fixture that adds a second literal or a cobra mutex fails the guard |
| PIB-504 | G | the authority path source (`internal/intentlock`) | `fstatfs`, `flock` and the unlock each execute inside a `SyscallConn().Control` callback on the retained directory `*os.File`; `(*os.File).Fd()` appears nowhere in the package; a sensitivity fixture reintroducing `syscall.Fstatfs(int(f.Fd()), …)` fails the guard |
| PIB-505 | G | every `§7.8 step N` and `§10.5 step N` reference in this PRD and ADR-035 | each resolves to the step whose text it describes; a semantic sensitivity fixture that renumbers a reference to a **real but wrong** step fails the guard |

### 18.45 AR — Rev-7 adjudication: exit-6 route partition, abandon totality and conditional retries

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-506 | I | the `archive-purge-evidence-divergent` refusal, in each of its sub-populations — a hash-wrong regular file under a **pending** reference; a symlink, a directory, a FIFO and a device node where a **pending** blob belongs; and an index that stopped strict-decoding mid-run | exit 6; the message names the pending hash and the repo-relative `blobs/<hash>.blob` and `index.json` paths, prints the explicit destructive warning **above** the command, names the single type-total `rm -rf -- <managed blob path>` (blob forms) or the restoration step and **no** removal at all (index form), names the sanitized rerun with `retry_cwd: "workspace-root"`, and states both the destructive cost and the Git-history caveat; no `cp` command and no wildcard appears in any of them; the string `--abandon-transaction` appears **nowhere** in it, and no absolute path appears anywhere in it; a tombstoned reference whose named blob is present produces exit **3** `archive-index-storage-inconsistent` instead and never this code, in **both** its globally-unreferenced and its still-live subcase (PIB-521, PIB-531) |
| PIB-507 | I | executing the documented blob-form procedure after PIB-506, once per object kind — run the printed `rm -rf --` verbatim against the managed blob path, then the sanitized rerun | the printed command succeeds on all five kinds (regular file, symlink, directory, FIFO, device node) with a shell exit of 0 and removes exactly that path, following no symlink to its target and touching no sibling; the rerun then takes the pending+absent branch of §9.7.2, exits 0 with `outcome: "recovered"`, tombstones every reference to that hash, removes nothing, and leaves an index that strict-decodes; a subsequent ordinary archive mutation proceeds (PIB-547) |
| PIB-508 | G | the exit-6 route map over the §10.4.1 catalog | every exit-6 code resolves to exactly one route — the nine journal/publication codes to `--abandon-transaction` (plus §6.6's manual fallback), `archive-purge-evidence-divergent` to §9.7.2's archive procedure; no code maps to zero routes or to two; a semantic sensitivity fixture that routes `archive-purge-evidence-divergent` to the abandon mode fails the guard |
| PIB-509 | I | `--abandon-transaction --yes` with journal evidence present and, in three separate fixtures, an **absent** feature directory, a malformed `status.json` and an unreadable `status.json` | exit 0 in all three; the evidence is moved; `feature-not-found`, `status-malformed` and `status-unreadable` are never emitted; `.tpatch/features/` is byte-identical (or still absent) afterwards |
| PIB-510 | C | abandon-mode read-ordering spy over the PIB-509 fixtures | between the flock and the abandon branch the command performs zero opens of `.tpatch/features/<slug>/`, zero `status.json` decodes and zero G1–G4 spawns; §10.5 step 6 is not entered in this mode |
| PIB-511 | G | §6.6's pre-abandon gate table against the implementation's control flow over the table's declared domain | the guard **derives** both the domain and the row set. The domain is **every argv that requests a *true* abandon** — the bare flag or `=true` — **plus every argv that names the flag and stops in the parse layer** on a pflag, arity or mutex error, so the parse/arity/mutex row is reachable by construction and is exercised by real fixtures (`--abandon-transaction --check`, `--abandon-transaction --dry-run`, `--check --abandon-transaction=false`, and a slugless invocation), while `prepare <slug> --abandon-transaction=false` — which selects `generate` (§5.2) — is **outside** it. The table's rows are exactly the stops that domain can reach before rule 4, each appearing once with its exit code, refusal code and route. It fails in **both** directions — a reachable stop missing from the table fails it, and a listed row for which no fixture in the domain can produce the refusal fails it too. Four semantic sensitivity fixtures: one inserts a new pre-branch refusal without adding its row; one adds a post-parse row for a mutex-incompatible branch, spelled and formatted correctly; one restates the domain as "syntactically valid invocations" while keeping the parse row, which the guard must reject as self-contradictory; one restates it as "every argv in which the flag appears", which the guard must reject because `=false` selects `generate` |
| PIB-512 | I | `--abandon-transaction` (with and without `--yes`) refused by `transaction-in-progress` while a live sibling holds the authority, with journal evidence present in the lane | exit 3; the message says the authority is held, the holder is unknowable and the action is to wait and retry; it names **no** manual removal, contains no `rm`, and the lane is byte-identical afterwards |
| PIB-513 | I | `--abandon-transaction --yes` with a J1–J10-failing journal **and** an absent feature directory **and** `git` absent from `PATH` | exit 0; the journal is moved and never decoded; no `journal-*`, `feature-not-found` or `local-lane-*` code is emitted; a process spy records zero Git spawns |
| PIB-514 | C | `feature intent-archive purge` and `feature intent-archive list`, process spy, over every selector and both the preview and `--yes` forms | zero Git processes are spawned in every combination; a filesystem spy shows purge writes only under `.tpatch/features/<slug>/artifacts/intent-archive/` and zero bytes under `.tpatch/local/` |
| PIB-515 | I | `purge` **without** `--yes`, no journal, one or more removal-pending hashes in the index — run once per selector (`--blob`, `--generation`, `--all`, `--orphans`) | exit 0 and `outcome: "recovery-required"` in every case; a filesystem spy records zero writes and an authority spy records zero flock acquisitions; the output states that pending purge recovery is required, lists each pending hash with its repo-relative blob and index paths and the per-hash plan, and names **the operator's own selector** with `--yes` appended, run from the workspace root; `outcome` is never `recovered` and nothing is finalized |
| PIB-516 | C | purge failure injected exactly between two hashes, after one hash fully completed and before the next begins (`failPurgeBetweenHashes`) | exit 5, `archive-purge-partial`; `resume: "completion-only"`; `pending_hash` absent; no reference is removal-pending; a single retry from the workspace root completes `remaining_hashes` at exit 0 and emits **no** `recovered` outcome; the report never promises one |
| PIB-517 | C | `purge --orphans --yes` failure injected after the first orphan removal (`failOrphanRemoveAfterFirst`) | exit 5, `archive-purge-partial`; `resume: "orphan-scan"`; `pending_hash` absent; `index.json` byte-identical to its pre-run bytes; a single retry removes exactly the remaining orphans at exit 0 and emits **no** `recovered` outcome |
| PIB-518 | G | every `purge_progress` object the command can emit | `pending_hash` is present if and only if `resume` is `pending-recovery-then-completion`; the retry prose matches the branch named by `resume`; a semantic sensitivity fixture whose `completion-only` or `orphan-scan` report promises a `recovered` run, or whose `pending-recovery-then-completion` report omits `pending_hash`, fails the guard |
| PIB-519 | G | every shipped string, help text and doc sentence this cluster owns that asserts a slug, archive or transaction cannot be permanently blocked | each names the command or the manual procedure that unblocks it; a semantic sensitivity fixture asserting "the slug is never permanently blocked" with no route named fails the guard |
| PIB-520 | S | §5.1's grammar block against the registered flag set | all twelve registered flags appear on at least one grammar line, each only in modes where it is legal, and `--allow-heuristic` appears on both the `generate` and `regenerate` lines; a flag registered but absent from every line fails the scan |

### 18.46 AS — Rev-8 adjudication: physical residue, purge-owned recovery and preview shape

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-521 | I | an index in which **every** reference to `h`, in every generation, is tombstoned — so `h` is globally unreferenced — while `blobs/<h>.blob` is present as a regular hash-correct file, observed by an ordinary mutating `prepare`, by `--regenerate`, by `feature intent-archive list` and by `doctor` | the mutating paths exit **3** with `refusal.code: "archive-index-storage-inconsistent"` and a whole-tree byte snapshot identical to the pre-run tree (zero removals, zero index writes, zero pending records); every one of the four surfaces classifies it as unreferenced physical residue and names the literal `tpatch feature intent-archive purge <slug> --orphans --yes` run from the workspace root; `list` exits **0** and renders the storage state as `orphan`; the strings `archive-purge-evidence-divergent` and `--abandon-transaction` appear nowhere, no pending hash is named, and no absolute path appears |
| PIB-522 | I | running exactly the command PIB-521 named, from the workspace root, over the same fixture | exit 0; the `--orphans --yes` selection is admitted past the X11 observation **after** a successful strict X1–X10 decode (a decode spy records the decode before the first removal); `blobs/<h>.blob` is gone; `index.json` is **byte-identical** to its pre-run bytes; the tombstone is unchanged; a following ordinary `--regenerate` that reproduces that generation proceeds and rehydrates through §9.3 |
| PIB-523 | I | `purge <slug> --orphans` **without** `--yes` over the PIB-521 fixture | exit 0; a filesystem spy records zero writes and an authority spy records zero flock acquisitions; the output names the repo-relative `blobs/<h>.blob` path, states that every reference to `h` is tombstoned, and names the `--orphans --yes` rerun under the one verbatim `Run this again from the same workspace root:` heading with `retry_cwd: "workspace-root"` |
| PIB-524 | G | the X11 storage-observation classification map over §9.3's wire-state table, whose domain is (wire state × blob observation × the hash's global **ownership** × its global **liveness**) | every reachable tuple resolves to exactly one classification and exactly one route: any tuple whose hash is **owned** → `recovery-pending`, routed to `feature intent-archive purge … --yes`, for every reader but that command; retained/absent → `archive-blob-dangling` + the confirmed `--blob <h> --yes`; retained/present-non-regular-or-hash-wrong → `archive-blob-corrupt` + §9.3.1's type-total removal-then-confirmed-purge or restore route; pending/unsafe-or-wrong → `archive-index-storage-inconsistent` + §9.7.2's archive procedure; tombstoned/present with the hash **unreferenced** → `archive-index-storage-inconsistent` + `--orphans --yes`; tombstoned/present with the hash **live but not owned** → `archive-index-storage-inconsistent` + the confirmed `--blob <h> --yes`, never `--orphans`; retained/absent with the hash **owned** → routed, never dangling; tombstoned/present-non-regular-or-hash-wrong → `archive-blob-corrupt` in both the unreferenced and the live-but-not-owned case, the corrupt classification dominating; no tuple maps to zero routes or to two, and the accepting tuples are unchanged. Four semantic sensitivity fixtures: one classifies either tombstoned/present tuple as `archive-purge-evidence-divergent`; one collapses the two tombstoned/present tuples into a single per-reference rule that routes both to `--orphans --yes`; one drops the ownership dimension so an owned hash is classified as mixed or dangling instead of routed; one routes a tombstoned/present **non-regular** file to `--orphans --yes` as residue instead of refusing it as corrupt — all spelled correctly and formatted validly, and all must fail |
| PIB-525 | I | a mutating `prepare` in each of `generate`, `manual` and `regenerate`, with one or more removal-pending archive references and **no** journal | exit **3**, `recovery-pending`; a whole-tree byte snapshot identical to the pre-run tree; a filesystem spy records zero blob removals and zero index writes; `outcome` is never `recovered` and no `recovery` object is emitted; the remediation names the sanitized `tpatch feature intent-archive purge <slug> --all --yes` (or `--blob <h> --yes` when one hash covers the pending set) with `retry_cwd: "workspace-root"` and no inherited `--path`; running the named command then exits 0 `recovered` and a further `prepare` proceeds |
| PIB-526 | G | the `RecoverPendingPurge` call graph | it has exactly **one** call site and that site is on the `feature intent-archive purge --yes` path; no mutating `prepare` mode, no preview path, no `list` and no `doctor` reaches it, asserted by a reverse call-graph walk rather than a name scan; a sensitivity fixture that adds a call from the publication path fails the guard |
| PIB-527 | I | `purge <slug> --orphans --yes` with one or more removal-pending hashes and no journal | exit 0, `outcome: "recovered"`, `recovery.kind: "archive-purge-finalize"`; the pending hashes are finalized; **no orphan is removed in that invocation**; the reported `retry` preserves `--orphans` verbatim with `retry_cwd: "workspace-root"`; re-running it removes exactly the orphans and exits 0 |
| PIB-528 | G | purge recovery and preview totality over the four selectors | for `--blob`, `--generation`, `--all` and `--orphans`: the `--yes` form finalizes any pending hash first and returns terminal `recovered` without processing the selector, and the preview form takes no lock, writes nothing and returns `recovery-required`; every emitted `retry` reproduces the operator's own selector and its own hashes/ids with `--yes` appended and no root-selection argv. A semantic sensitivity fixture whose `--orphans` preview reports a `--all --yes` retry — valid JSON, correct field names, a real command — fails the guard |
| PIB-529 | I | the pending-purge preview report, JSON and human, for each selector | JSON carries `outcome: "recovery-required"`, `action: "none"` and a `pending_purge` object with exactly `recovery_required: true`, `pending_hashes[]` (each `hash`, repo-relative `blob`, repo-relative `index`, fixed `plan` naming the claim, the conditional removal and the tombstone of every reference), the closed `selector`, `retry` and `retry_cwd: "workspace-root"`, in that key order, with `pending_hashes[]` sorted lexically and never null; the human rendering carries the same hashes, the same two repo-relative paths, the same plan sentence and the same retry line under the one verbatim `Run this again from the same workspace root:` heading this cluster uses everywhere, emitted at column 0 with no leading or trailing whitespace (§9.7.2, PIB-498); no `recovery` object, no `purge_progress`, no `archive` and no absolute path appear in either |
| PIB-530 | I | `feature intent-archive purge`, every selector, over three fixtures — a clean archive, a removal-pending archive, and one with a pending prepare journal — in both the preview and the `--yes` form | an authority spy records **zero** flock acquisitions for every preview combination and exactly **one** for every `--yes` combination; no cache, lock file or other durable authority artifact is created by either form; a process spy records zero Git spawns in all of them |

### 18.47 AT — Rev-9 adjudication: global-hash residue, retry heading parity and gate domain

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-531 | I | an index in which generation `G1` references `h` as **retained** while generation `G2` references the same `h` as **tombstoned**, with `blobs/<h>.blob` present as a regular hash-correct file, observed by an ordinary mutating `prepare`, by `--regenerate`, by `feature intent-archive list` and by `doctor` | the mutating paths exit **3** with `refusal.code: "archive-index-storage-inconsistent"` and a whole-tree byte snapshot identical to the pre-run tree; `list` exits **3** and renders the storage state as `mixed-reference`, naming the repo-relative `blobs/<h>.blob` path, the tombstoning generation ids and the live generation ids; all four surfaces classify it as a mixed tombstone/live-reference global-availability inconsistency and name the literal `tpatch feature intent-archive purge <slug> --blob <h> --yes` run from the workspace root; the strings `--orphans`, `archive-purge-evidence-divergent` and `--abandon-transaction` appear **nowhere** in any of the four outputs, and no absolute path appears |
| PIB-532 | I | running exactly the command PIB-531 named, from the workspace root, over the same fixture | exit 0; the mixed pair is admitted past the X11 observation **after** a successful strict X1–X10 decode (a decode spy records the decode before the first mutation); every reference to `h` in every generation is tombstoned afterwards, `blobs/<h>.blob` is gone, `index.json` strict-decodes and X11 is satisfied; **both** the retained `G1` reference **and** the already-tombstoned `G2` reference passed through removal-pending before the removal — an index-write spy records one claim rewrite in which *every* reference to `h`, including the tombstoned one, carries `purge_pending: true`, then the removal, then the tombstone rewrite, in that order — so no same-hash reference is a claim no-op; no reference is left retained and none is left dangling |
| PIB-533 | G | the `--orphans` selection set, derived from the implementation's orphan predicate rather than from its output | the predicate is **global by content hash**: a blob is selected only when no reference to its hash, in any generation, is retained or removal-pending. Over a fixture holding a directory-scan orphan and a globally-unreferenced tombstoned hash and **no** other inconsistency, `purge --orphans --yes` removes exactly those two and the resulting index leaves no dangling reference. Over the same fixture **plus** PIB-531's mixed hash, the whole invocation instead refuses exit 3 zero-write naming the mixed hash's `--blob <h> --yes` repair — a filesystem spy records zero removals, including of the two blobs the first fixture proved removable — because X11's scan is global and no partial cleanup proceeds around an inconsistency (PIB-542). Two semantic sensitivity fixtures, both valid code with correct names: one whose predicate is evaluated per reference, so a tombstoned reference admits its blob to the orphan set while another generation still retains the hash; one that removes the removable orphans and reports the mixed hash as a leftover |
| PIB-534 | I | an archive holding both a removal-pending reference to `h₁` **and** a tombstone-beside-present-blob observation on `h₂`, run for each of `--orphans` and `--blob <h₂>`, in preview then `--yes` then rerun | the preview exits 0 `recovery-required` naming only `h₁`, with an authority spy recording zero flock acquisitions and a filesystem spy zero writes; the `--yes` run exits 0 `recovered` with `recovery.kind: "archive-purge-finalize"`, finalizes `h₁` only, leaves `h₂`'s blob and index entry byte-identical, and reports the operator's own selector as the retry — **it is not refused by `h₂`'s inconsistency**, which is the recovery exception of §9.3.1 asserted at the composition level and cross-checked against PIB-546's order spy; the rerun then performs the correct `h₂` repair for its subcase at exit 0, or — for a selector that is not that subcase's admitted repair — refuses exit 3 naming that subcase's literal repair command, which then completes at exit 0. No step in the ladder returns a refusal whose remediation names no executable command, and at no point is `h₂` acted on by an admission earned by a different repair class (PIB-542, PIB-546, PIB-549) |
| PIB-535 | I | `feature intent-archive list <slug>` over five fixtures — a clean archive, a globally-unreferenced tombstone-beside-blob, a mixed tombstone/live-reference hash, a dangling retained reference, and a retained reference beside a present hash-wrong blob | exit **0** for the clean archive and for the unreferenced residue (rendered `orphan`, carrying the literal `--orphans --yes` repair); exit **3** for the mixed hash (rendered `mixed-reference`, carrying the literal `--blob <h> --yes` repair), for the dangling reference (rendered `dangling`, carrying the literal `--blob <h> --yes` repair) and for the corrupt blob (rendered `corrupt`, carrying §9.3.1's type-total removal-then-confirmed-purge or restore route); a filesystem spy records zero writes and an authority spy zero flock acquisitions in all five; no absolute path, no symlink target and no repair string other than the one its subcase owns appears in any of them. Coexisting observations are covered by PIB-541, and multiple instances of one class by PIB-548 |
| PIB-536 | C | a mutating `prepare` whose workspace `*os.Root` open fails and, in a second fixture, whose `root.Open(".")` fails, each injected at `beforeLockAcquire` before any `fstatfs` or `flock` is attempted | exit **3**, `refusal.code: "directory-flock-unavailable"`, with a whole-tree byte snapshot identical to the pre-run tree; a syscall spy records zero `fstatfs` and zero `flock` calls; the codes `transaction-in-progress`, `lock-filesystem-unsupported` and `workspace-not-initialized` are never emitted; with journal, preimage or staging evidence present for the slug the message additionally names the repo-relative `.tpatch/local/intent-prepare/<slug>/` lane and the last-resort manual removal exactly as §6.6's gate row 8 requires, and no absolute path appears |

### 18.48 AU — Rev-10 adjudication: global pending ownership, selector-independent validation and the corrupt-blob route

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-537 | I | the boolean domain of `--abandon-transaction`, over four real invocations: `prepare <slug> --abandon-transaction`, `--abandon-transaction=true`, `--abandon-transaction=false`, and `--check --abandon-transaction=false` | the first two select `abandon` and take §6.6's contract (preview, exit 0, nothing moved); the third selects **`generate`** and takes the default mode's whole precedence ladder — a mode spy records `generate`, the lane is never opened for abandon, and no `no-pending-transaction` or `abandon-*` code can be emitted; the fourth exits **1** at pflag's presence-keyed mutex with pflag's own text and zero writes. The third invocation is outside §6.6's gate-table domain and the fourth is inside it at row 1, asserted against the guard's own domain predicate rather than against prose (§5.2, §6.6, PIB-511) |
| PIB-538 | I | `prepare <slug> --manual` — the mode that writes no archive — over five archive fixtures: one removal-pending reference; a dangling retained reference; a globally unreferenced tombstone-beside-blob; a mixed tombstone/live-reference hash; and a retained reference beside a hash-wrong blob | each refuses at exit **3** with the code §9.3.1/§7.8 step 5 assigns (`recovery-pending`, `archive-blob-dangling`, `archive-index-storage-inconsistent` twice, `archive-blob-corrupt`) and names that observation's owning repair verbatim; in every one a whole-tree byte snapshot is identical to the pre-run tree, a provider spy records zero calls, a filesystem spy records zero writes, `status.json` is byte-identical and no `FEATURES.md` refresh occurs. The refusal fires even though the mode touches no blob and no index entry, and the message never suggests that `--manual` can finalize or repair archive state (§6.2) |
| PIB-539 | I | `feature intent-archive purge <slug> --blob <h> --yes` over an index where generation `G1` references `h` as **retained**, `G2` references `h` as **removal-pending**, and `blobs/<h>.blob` is present, regular and hash-correct | exit 0 `recovered`; an index-write spy records exactly this order — one **claim** rewrite in which *every* reference to `h` is removal-pending, then the blob removal, then one tombstone rewrite in which *every* reference to `h` is tombstoned; a filesystem spy records **zero** blob removals before the claim rewrite lands; afterwards no reference to `h` is retained, none is dangling, and X11 is satisfied. The same fixture observed by an ordinary mutating `prepare`, by a `purge` preview and by `list` reports the hash as pending and routes to this command — `recovery-pending`, `recovery-required` and `pending-remove` respectively — and never as dangling, residue, mixed or corrupt, which is the ownership-outranks rule asserted directly (§9.3, §9.7.2) |
| PIB-540 | C | crash injection at each point of the claimed per-hash machine over the PIB-539 fixture — `beforePurgeIndexCAS` (before the claim), `beforePurgeBlobRemove` (after the claim), `afterPurgeBlobRemove` and `beforePendingTombstoneCAS` — each followed by a rerun of the same sanitized command | after every crash the invariant holds on disk: `blobs/<h>.blob` is present **or** no reference to `h` is retained, never both absent-and-retained by tpatch's own action; each rerun performs exactly the one next action (re-claim, revalidate-and-remove, tombstone-all) and terminates at exit 0 `recovered`; the post-removal crash resumes through the absent-blob path and tombstones every reference to `h` including `G1`'s, performing zero removals; no rerun leaves a dangling reference and no rerun requires a second distinct repair command (§7.10 CP12/CP12a, §9.7.2) |
| PIB-541 | I | `feature intent-archive list <slug>` and `doctor` over one archive holding **both** a globally unreferenced tombstone-beside-blob (an exit-0 observation) and a mixed tombstone/live-reference hash (an exit-3 observation), plus a clean generation | `list` exits **3**, and its body renders **all three** observations — the clean references, the `orphan` with its literal `--orphans --yes` repair, and the `mixed-reference` with its literal `--blob <h> --yes` repair — with neither the exit-0 observation suppressed nor the report truncated at the exit-3 one; `doctor` reports the identical set with the identical repairs, warning-only, and a filesystem spy records zero writes and an authority spy zero flock acquisitions in both. Reversing the fixture's generation order changes neither the exit code nor the rendered set, so the exit is a property of the observations and not of iteration order (§9.7, §12.5) |
| PIB-542 | I | `feature intent-archive purge <slug> --yes` on **four disjoint selectors** — `--orphans`, `--blob <h₃>`, `--generation <id₃>` and `--all` — over an archive whose *unrelated* hash `h₂` is a mixed tombstone/live-reference inconsistency, where `h₃`/`id₃` provably touch no reference to `h₂` and a removable orphan is also present | every one of the four exits **3** `archive-index-storage-inconsistent` with a whole-tree byte snapshot identical to the pre-run tree: the orphan is **not** removed, `h₃` is **not** purged, no pending record is written and `index.json` is byte-identical; each message names `h₂`'s literal `--blob <h₂> --yes` repair rather than the selector the operator typed; running that repair and then re-running the original selector completes at exit 0. A decode spy confirms the whole-index X11 scan ran before the refusal, and a second fixture in which `h₂` is clean shows all four selectors succeeding, so the refusal is attributable to `h₂` alone. A third fixture holds **two** mixed hashes and shows that `--blob <h₂> --blob <h₂'> --yes` — the class fully covered — is admitted and repairs both, while `--blob <h₂> --yes` alone is refused zero-write for leaving an instance of its own class uncovered (§9.3.1, §9.7.1, §9.7.2, PIB-548, PIB-549) |
| PIB-543 | I | a **retained** reference whose `blobs/<h>.blob` is present but (i) a symlink, (ii) a directory, (iii) a regular file whose bytes hash to something else, (iv) a FIFO and (v) a device node — observed by an ordinary mutating `prepare`, by `--regenerate`, by `--manual`, by `list` and by `doctor`, then repaired | every observer exits **3** `archive-blob-corrupt` with a whole-tree byte snapshot identical to the pre-run tree; `list` renders the storage state `corrupt` and exits 3; all five name one repo-relative procedure — the explicit destructive warning, the single `rm -rf -- <managed blob path>`, then, under the one column-0 retry heading, the literal `tpatch feature intent-archive purge <slug> --blob <h> --yes` with `retry_cwd: "workspace-root"` — plus the restore-the-correct-blob alternative, the destructive cost and the Git-history caveat, with no absolute path anywhere, no `--orphans` mentioned and no `cp` command emitted. Executing the procedure completes on **all five** object kinds: after the removal the reference is dangling, the confirmed purge is admitted past X11, tombstones every reference to `h`, performs zero removals and exits 0, and a following ordinary mutation proceeds. Writing the correct blob back instead also clears it with nothing purged (§9.3.1, §9.7.3, §10.7, PIB-547) |
| PIB-544 | C | an external same-hash reference inserted into `index.json` at each of the four insertion windows of §9.7.2 — before the claim's re-read, between the re-read and its CAS, between the CAS and the pre-removal revalidation, and after the removal — driven through `beforePurgeIndexCAS`, `afterPurgeBlobRevalidate`, `beforePurgeBlobRemove` and `beforePendingTombstoneCAS` (the fifth window, external **replacement** of the managed object between revalidation and unlink, is PIB-550) | window 1 completes normally with the inserted reference claimed and tombstoned like every other; windows 2 and 3 perform **no** removal and end in `archive-purge-partial` at exit 5 with `resume: "pending-recovery-then-completion"` and `pending_hash: h` — the pending record already existed, so the post-first-mutation window is open — and the reported sanitized retry re-derives the claim and completes at exit 0; window 4 is detected by the tombstone CAS and its retry takes the absent-blob path, tombstoning the inserted reference with the rest. A fifth fixture inserts the reference against a **new** selection before its first write and asserts exit 3 `archive-purge-index-changed` with a whole-tree byte snapshot. In no window is a blob removed while a non-pending reference to it is visible in the index the command validated, and in no window does the command loop in-process (§9.7.2, §7.12) |
| PIB-545 | G | the archive's storage-classification, **claim** and removal authority, **derived** from the implementation rather than from a table: the map over (wire state × blob observation × the hash's global **ownership** × its global **liveness**), the domain of the claim CAS, and the guarded precondition of every blob removal | every reachable tuple resolves to exactly one disposition and one route: any tuple whose hash is **owned** resolves to `recovery-pending` for every reader except `purge --yes`, which claims it; retained/absent → `archive-blob-dangling`; retained/present-unsafe-or-wrong → `archive-blob-corrupt` with §9.3.1's type-total procedure; pending/unsafe-or-wrong → `archive-purge-evidence-divergent`; tombstoned/present-regular-hash-correct with the hash unreferenced → `--orphans --yes`; tombstoned/present-regular-hash-correct with the hash live-but-not-owned → the confirmed `--blob <h> --yes`; tombstoned/present-unsafe-or-wrong → `archive-blob-corrupt`, the corrupt classification dominating both; accepting tuples unchanged. The guard additionally asserts, by control-flow walk rather than name scan, that (a) the claim CAS's reference set is **every** reference with the target hash — retained and tombstoned alike, with no wire-state predicate excluding any of them — and (b) **every** blob-removal call site is dominated by a check that no reference to that hash is anything but removal-pending. Six semantic sensitivity fixtures, each spelled correctly and formatted validly, must fail: one that classifies an owned hash as mixed or dangling instead of routing it; one that removes a blob under a hash with a surviving retained reference; one that maps retained/present-hash-wrong to `archive-blob-dangling`; one that admits `--orphans --yes` past a mixed hash; **one whose claim CAS skips already-tombstoned references while the removal gate still requires every reference to be pending — the deadlock rev-10 shipped in prose**; and one whose absent-blob path tombstones only the pending references and leaves a retained one behind |

### 18.49 AV — Rev-11 adjudication: total same-hash claim, the recovery exception, type-total removal and repair-class multiplicity

| ID | Kind | Case | Asserted observable |
|---|---|---|---|
| PIB-546 | G | the ordering of pending-purge recovery against the whole-index X11 scan, **derived** from the implementation's control flow rather than from prose: every archive-mutating entry point, the position of its global storage validation, and the position of `RecoverPendingPurge` | a control-flow walk shows exactly one path that mutates before the global X11 scan, and it is `RecoverPendingPurge` on `feature intent-archive purge --yes`, restricted to hashes the index already marks removal-pending; every other archive mutation — each mutating `prepare` mode and each new-selector purge — is dominated by that scan. Run against an archive holding a removal-pending `h₁` **and** an unrelated mixed hash `h₂`, `purge <slug> --orphans --yes` and `purge <slug> --blob <h₂> --yes` both exit **0** `recovered`, finalizing `h₁` while `h₂` is byte-identical, and neither processes its selector; each rerun then performs the full global scan and refuses or repairs per §9.3.1. Three semantic sensitivity fixtures, each valid code with correct names, must fail: one that runs the global scan first and so refuses the recovery, leaving `h₁` pending with the refusing command as its only named repair; one whose recovery continues into the selector after finalizing; and one that grants the same exception to a mutating `prepare`, which must keep refusing exit 3 `recovery-pending` (§9.3.1, §9.7.2, §10.5 step 22) |
| PIB-547 | I | the corrupt-object and divergent-blob procedures as **printed strings**, executed verbatim from the workspace root against a managed `blobs/<hash>.blob` that is in turn a hash-wrong regular file, a symlink to a file outside `.tpatch/`, a directory containing two files, a FIFO and a device node | in all five the printed removal is the single line `rm -rf -- <the repo-relative managed blob path>`, preceded by the explicit destructive warning on its own line(s) and containing no wildcard, no second path, no directory prefix above the blob and no `-i`/`-I` interactive form; running it exits 0 on Linux and Darwin, leaves the path absent, leaves the symlink's **target** untouched, and removes no sibling blob. No `cp`, `mv`, `rsync`, `tar` or other copy command appears in any of the five outputs; instead the message states that preserving the object requires kind-appropriate tooling chosen by the operator. The index-divergence form of the same code names **no** removal command at all and no managed blob path. A sensitivity fixture that emits rev-10's `cp` + plain `rm` pair, and one that emits `rm -rf <path>` without the `--` terminator, both fail (§9.3.1, §9.7.2, §10.7, PIB-506, PIB-543) |
| PIB-548 | I | multi-instance repair classes, one class per fixture: (a) three globally unreferenced tombstone-beside-blob residues; (b) two dangling retained hashes; (c) two mixed tombstone/live-reference hashes — each observed by `list` and `doctor`, then repaired by one invocation | `list` and `doctor` render every instance, each carrying the **same** class repair for (a) and its own `--blob <h> --yes` for (b) and (c), and neither implies one run per instance; one `purge <slug> --orphans --yes` removes all three residues in (a) at exit 0 with `index.json` byte-identical; one `purge <slug> --blob <h₁> --blob <h₂> --yes` completes (b) with zero removals and (c) with two removals, both at exit 0, after which X11 is satisfied and an ordinary mutation proceeds. A fourth fixture mixing classes (a) and (c) refuses every selector at exit 3 zero-write and renders both routes (§9.7.1, §9.7.3, PIB-542, PIB-549) |
| PIB-549 | G | the admission predicate for X11-refused states, **derived** from the implementation: the mapping from (observed inconsistency set, confirmed selection) to admitted/refused | admission is granted exactly when the observed set is non-empty, belongs to a **single** repair class, and the confirmed selection covers **every** instance of that class; it is refused when two classes are present, when an instance of the admitted class is not covered by the selection, and when the selection is not that class's shipped repair. `--orphans --yes` is admitted only for the unreferenced-residue class; a selection total over each dangling hash only for the dangling class; a selection total over each mixed hash only for the mixed class; no selection is ever admitted past a corrupt object or an unsafe pending blob. Three semantic sensitivity fixtures, all valid code with correct names: one that keeps rev-10's "sole inconsistency" rule and so refuses an archive holding two residues, leaving it with no shipped repair; one that admits a selection covering only part of its class and repairs it partially; one that admits a selection past a second class because the first was admitted (§9.3.1, §9.7.2) |
| PIB-550 | C | the revalidate→unlink window: an external writer replaces the object at `blobs/<h>.blob` between the pre-removal revalidation and the unlink, injected at `beforePurgeBlobRemove` after `afterPurgeBlobRevalidate` has passed, over both a replacement regular file and a replacement directory | the unlink removes whatever is at that path, the machine continues to the tombstone CAS, and the invocation ends at exit 0 `recovered` with every reference to `h` tombstoned — an index-consistent outcome whose byte-level loss of the replacement is **not** detected. The guard half asserts the disclosure rather than the behavior: no shipped string, and no sentence of this PRD or ADR-035, claims this window is closed, conditioned or detected, and a sensitivity fixture that adds "the removal is conditioned on the revalidated content" to either document or to a report must fail. The residual is rendered beside the post-CAS final-syscall race in the same disclosure, never in place of it (§7.1, §9.7.2) |
| PIB-551 | G | the wire/storage disposition table of §9.3 against the implementation, over the **full** domain (wire state × blob observation ∈ {absent, present-regular-hash-correct, present-non-regular-or-hash-wrong} × ownership × liveness) | every reachable triple resolves to exactly one row of the shipped table and exactly one route, and the guard derives both sides — the domain from the enumerated wire states and blob observations, the mapping from the classifier — rather than reading the document. No triple maps to zero routes, none maps to two, and the three retained rows (present-hash-correct, absent, present-unidentifiable) each split correctly on ownership. Two semantic sensitivity fixtures: one that drops the retained/absent-and-owned triple so an owned dangling reference is classified as `archive-blob-dangling` instead of routed to its transaction; one that adds a second route to the tombstoned/present-unidentifiable triple by offering both `--orphans --yes` and the corrupt procedure (§9.3, §9.3.1) |

### 18.50 Counts, kinds and slice partition

- **551 rows**, `PIB-001`…`PIB-551`, contiguous, zero duplicates, zero retired.
  160 rows are new in rev-1 (`PIB-235`…`PIB-394`); 15 are new in rev-2
  (`PIB-395`…`PIB-409`); **23** are new in rev-3 (`PIB-410`…`PIB-432`);
  **16** are new in rev-4 (`PIB-433`…`PIB-448`); **34** are new in rev-5
  (`PIB-449`…`PIB-482`); **23** are new in rev-6 (`PIB-483`…`PIB-505`);
  **15** are new in rev-7 (`PIB-506`…`PIB-520`);
  **10** are new in rev-8 (`PIB-521`…`PIB-530`);
  **6** are new in rev-9 (`PIB-531`…`PIB-536`);
  **9** are new in rev-10 (`PIB-537`…`PIB-545`);
  **6** are new in rev-11 (`PIB-546`…`PIB-551`);
  earlier IDs retain their
  numbers when amended (§18.1).
- **48 categories**: A 20, B 24, C 15, D 12, E 9, F 19, G 13, H 14, I 13, J 8,
  K 12, L 10, M 14, N 14, O 10, P 7, Q 6, R 3, S 9, T 2, U 10, V 12, W 5, X 6,
  Y 7, Z 4, AA 15, AB 7, AC 10, AD 8, AE 10, AF 5, AG 14, AH 17, AI 6, AJ 10,
  AK 10, AL 4, AM 15, AN 23, AO 16, AP 34, AQ 23, AR 15, AS 10, AT 6, AU 9,
  AV 6.
  **48 categories; sum = 551.**
- **By kind**: `I` 242, `C` 122, `G` 113, `U` 49, `S` 25. Sum = 551. (Rev-11 adds
  `I` 2, `C` 1 and `G` 3 and re-kinds nothing, so the rev-10 totals of
  `I` 240, `C` 121, `G` 110, `U` 49, `S` 25 carry forward unchanged before the
  six new rows are added. Rev-10 added
  `I` 6, `C` 2 and `G` 1, and re-kinded nothing, so the rev-9 totals of
  `I` 234, `C` 119, `G` 109, `U` 49, `S` 25 carry forward unchanged before the
  nine new rows are added. Rev-9 added
  `I` 4, `C` 1 and `G` 1 and re-kinded exactly one stable row: `PIB-498` moved
  from `I` to `G`, because it is now the derived totality guard over every retry
  emitter rather than one path's assertion — so the rev-8 baseline of
  `I` 231 / `G` 107 became `I` 230 / `G` 108 before those six rows were
  added. Rev-8 added
  `I` 7 and `G` 3 and re-kinded nothing; the rev-7 baseline was
  `I` 224, `C` 118, `G` 104, `U` 49, `S` 25, which itself added
  `I` 6, `C` 4, `G` 4 and `S` 1 over rev-6's
  `I` 218, `C` 114, `G` 100, `U` 49, `S` 24, which added
  `I` 14, `C` 3 and `G` 6 over rev-5's `I` 204,
  `C` 111, `G` 94, `U` 49, `S` 24 — that revision re-kinded `PIB-445` from `C`
  to `G` because the probe it drove no longer exists.) Every row
  whose kind is `G` is covered by §18.51's sensitivity requirement.
- **Slice partition** (each row in exactly one slice):

| Slice | Categories | Rows |
|---|---|---|
| S1 transaction core | F, G, H, Z, AB, AC, AD | 75 |
| S1b directory authority | AA | 15 |
| S2 generator extraction | N, AJ | 24 |
| S3 archive | D, L, X, AG | 42 |
| S4 CLI wiring | A, B, C, E, I, M, T, U, V, W, Y, AF, AI | 142 |
| S4b retention surface | AH | 17 |
| S5 doctor, compatibility, non-invalidation | J, O, P, R, AE, AK | 48 |
| S6 docs, skills, guards | K, Q, S, AL | 31 |
| S7 rev-3…rev-11 cross-cutting hardening | AM, AN, AO, AP, AQ, AR, AS, AT, AU, AV | 157 |

Sum = 551; zero unassigned, zero double-assigned (PIB-230's ledger check
derives this partition mechanically rather than trusting the table).

### 18.51 Sensitivity requirement

Every row whose Kind contains `G` carries a **sensitivity fixture**: a
deliberately wrong input that the guard must reject. PIB-550 is kinded `C`
because its subject is an injected crash-window race, but it carries a
document-and-string disclosure assertion with its own wrong-input fixture, and
PIB-231 counts it through the Kind column exactly as it counts every other row —
the fixture is stated in the row rather than in the table below. A byte-scanning or
name-matching guard can false-pass silently, and the repository has already
been burned by exactly that
(`docs/prds/PRD-artifact-validation-and-provenance.md:3960-3991`). PIB-231 is
the meta-check that derives the guard set mechanically from the Kind column
rather than from a hand-maintained list.

**Semantic guards additionally require a semantic fixture.** Twenty-six guards
in this matrix assert a *meaning*, not a token, and a byte scan cannot establish
them.
Each therefore carries a fixture that is textually valid and semantically
wrong, and the guard must still fail:

| Guard | Semantic fixture that must fail |
|---|---|
| PIB-155 over-claim | a sentence that says "the six files appear together to any reader" without using the word "atomic" |
| PIB-244 flag-delta completeness | a delta table that lists every flag but describes one flag's *before* behavior incorrectly |
| PIB-250 coherence totality | a table covering all eight combinations where one outcome contradicts §6.1.2 |
| PIB-267 redaction non-override | a help string offering "skip the scan for trusted repositories" without the token `--force` |
| PIB-342 clone-durability | a sentence claiming durable recovery that never uses the word "clone" |
| PIB-356 availability truth | a `list` rendering that reports a hash as unavailable while another generation still retains it, and one that names `--orphans --yes` as the repair for a live blob |
| PIB-391 golden provenance | a golden file with the right bytes committed by the wrong commit range |
| PIB-459 single dangling repair | a remediation that offers "supply the original bytes and re-run" without using the word "rehydrate" |
| PIB-471 holder identity | a diagnostic string that says "no prepare is currently running" without naming a lock, holder or probe |
| PIB-477 provenance schema scope | a wire struct whose key is spelled `produced_by` rather than `generator`, and a canonical-prose fixture containing the word `generator` that must **pass** |
| PIB-487 post-recovery exit 3 | a code path that recovers, then evaluates the lifecycle gate and returns exit 3 with a message that truthfully says "wrote nothing new since recovery" |
| PIB-496 qualified recoverability | a sentence that says "any interrupted prepare can always be unblocked" without using the words "abandon", "platform" or "filesystem" |
| PIB-497 no inherited absolute path | a report that renders the absolute workspace path inside a prose sentence rather than in a `path`-named field |
| PIB-498 one verbatim retry heading | an emitter that prints `Run this again from the same workspace root to perform it:` — rev-8's own second spelling — and one that prints a numbered `3. run this again from the same workspace root:` step, both above a correct `retry` line; **and** an emitter that prints the correct sentence indented by two spaces, and one that prints it with a trailing space, both of which are byte-wrong at the line level a harness matches on |
| PIB-505 step references | a `§10.5 step N` reference renumbered to a **real but wrong** step whose text does not match the sentence citing it |
| PIB-508 exit-6 route partition | a catalog in which `archive-purge-evidence-divergent` is routed to `--abandon-transaction`, spelled correctly and formatted validly |
| PIB-511 pre-abandon gate totality | a pre-branch refusal added to the control flow whose code is already used elsewhere in the table, so a code-presence check would pass; **and** a post-parse table row added for a `--check`/`--dry-run`/`--yes`-validation branch this mode's grammar excludes, written in the table's exact column shape; **and** a domain restated as "syntactically valid invocations" while the parse/arity/mutex row is kept, which the guard must reject as self-contradictory; **and** a domain restated as "every argv in which the flag appears", which pulls `--abandon-transaction=false` — an invocation that selects `generate` — into a table of abandon's stops |
| PIB-518 conditional purge retry | a `completion-only` report whose prose says "the first run recovers, the second completes", with every field spelled and typed correctly |
| PIB-519 qualified recoverability | a sentence that says "no interrupted purge can leave the archive permanently blocked" without naming the abandon mode, the manual procedure or the rerun |
| PIB-524 X11 classification map | a map that files a tombstoned reference whose named blob is present under `archive-purge-evidence-divergent`, with the code spelled correctly and every other triple classified correctly; **and** a map that collapses the two tombstoned/present triples into one per-reference rule routing both to `--orphans --yes`; **and** a map that classifies an **owned** hash — one with a removal-pending reference — as one of the tombstoned/present subcases instead of routing it to its purge transaction |
| PIB-528 selector totality | an `--orphans` preview whose reported retry is `--all --yes` — valid JSON, correct field names, a real shipped command, and wrong |
| PIB-533 global orphan predicate | an orphan predicate evaluated per reference, so a tombstoned reference admits its blob to the orphan set while another generation still retains the same hash — valid code, correct names, and it deletes live data; **and** an `--orphans --yes` that removes the orphans it can while an unrelated mixed hash goes unrepaired, which is a partial cleanup around a global inconsistency |
| PIB-545 global ownership, claim domain and removal authority | a classification that files an **owned** hash — one with a removal-pending reference — as mixed, dangling or residue instead of routing it to its purge transaction; a blob removal whose call site is not dominated by a check that every reference to that hash is removal-pending, so a retained reference survives the removal; a map sending retained/present-but-hash-wrong to `archive-blob-dangling` rather than `archive-blob-corrupt`; an admission of `--orphans --yes` past a mixed tombstone/live-reference hash; **a claim CAS that skips already-tombstoned same-hash references while the removal gate still requires every reference to be pending, which deadlocks the hash forever**; and an absent-blob path that tombstones only the pending references and leaves a retained one behind. All six are valid code with correct names, and all six must fail |
| PIB-546 recovery-exception ordering | a control flow that runs the whole-index X11 scan before `RecoverPendingPurge`, so a pending hash cannot be finalized while an unrelated hash is inconsistent and the refusing command is the only repair the refusal names; a recovery that continues into the selector after finalizing; and a mutating `prepare` granted the same exception instead of refusing `recovery-pending`. All three are valid code with correct names |
| PIB-549 class-coverage admission | an admission predicate that keeps rev-10's "sole inconsistency" rule, so an archive holding two unreferenced residues is refused under every selector and has no shipped repair at all; one that admits a selection covering only part of its class and repairs it partially; and one that admits a selection past a second repair class because the first was admitted |
| PIB-551 wire/storage disposition totality | a table that drops the retained/absent-and-**owned** triple, so an owned dangling reference is classified `archive-blob-dangling` instead of routed to its transaction; and one that gives the tombstoned/present-unidentifiable triple two routes by offering both `--orphans --yes` and the corrupt procedure |

PIB-231 derives the guard set mechanically; this table is what stops twenty-six
of those guards from being satisfied by a spelling check (PIB-231's own
sensitivity fixture covers the derivation).

## 19. Implementation authorization gate

**No implementation is authorized by this document.**

1. This PRD must be accepted.
2. **ADR-035 must be accepted.** It is `Proposed` at rev-11, and a writer cannot
   accept its own ADR; the two are reviewed together and acceptance of both is
   the precondition for dispatching any slice.
3. **The accepted `prepare --check` PRD must be implemented, landed on
   `origin/main`, and passing its own acceptance matrix** — §17.1. Its goldens
   must come from that implementation's commit range, not from this cluster
   (PIB-391).
4. Only then may the cluster lead declare the §17.2 file partition and
   dispatch, beginning with S1b's pre-change `feature resource` goldens.

Until (1), (2) and (3) all hold, no file under `cmd/`, `internal/`, `assets/`,
`tests/`, and no change to `SPEC.md` or `CHANGELOG.md`, is authorized by this
PRD.

## 20. Open questions

Only genuinely unresolved items. Everything else is a decision. None blocks
review.

| # | Question | Why open | Default if unanswered |
|---|---|---|---|
| Q1 | Should `--regenerate --only <ids>` exist later? | Coherence argues for all-or-nothing (§6.3), but an operator who only wants a new `exploration.md` currently has no route short of deleting the file. | Not in v1; additive later with an enumerated coherence statement. |
| Q2 | Is `180s` the right whole-command deadline? | No distribution of three-call prepare durations has been measured; the per-phase `60s` (`internal/cli/cobra.go:629`) was not chosen empirically either. | `180s`, overridable; changing it moves only the fallback boundary. |
| Q3 | Should a `--require-provider` flag exist for the **default** mode? | `--regenerate` already requires one (§11.3.2), and default-mode fallback only ever creates absent files — but an operator who wants provider-or-nothing everywhere has no switch. | No flag in v1; the advisories name every fallback. |
| Q4 | Should a `tpatch prepare <slug> --restore <hash>` verb exist? | `cp` is unambiguous and needs no new surface, but it is unguarded — nothing stops restoring a blob over the wrong path. | No verb in v1; the report prints the exact `cp` form. |
| Q5 | Should purge gain an **age- or count-based** retention policy? | §9.7 ships explicit, operator-driven purging; an automatic policy would need a clock, which every determinism rule in this cluster forbids in tracked artifacts. | Explicit purge only; any automatic policy needs its own determinism argument. |
| Q6 | Should `--regenerate` invalidate a `verified-fresh` label? | Intent changed, so the freshness claim is arguably stale — but ADR-013 makes `verify`/`amend` the only writers of that record, and `define` has the same effect today without invalidating. | No change; widening `verify`'s writer set is a separate PRD with its own delta. |
| Q7 | Should the archive be per-feature (chosen) or per-workspace with cross-feature dedupe? | A workspace-level blob store would dedupe across features, but it couples feature directories, complicates `land`'s per-feature path set (`internal/cli/land.go:723-725`) and makes `tpatch remove` ambiguous. | Per-feature, as specified. |
| Q8 | Should `doctor` D9 be an error rather than a warning? | A pending journal is recoverable automatically, so failing `doctor` would over-alarm — but a journal that persists for weeks indicates an abandoned transaction. | Warning; a future PRD may add an age-based escalation, which would require a clock and therefore its own determinism argument. |
| Q9 | Should the redaction gate ever be overridable? | A false positive on `home-absolute-path` or the broad `email-pii` class inside legitimate Markdown blocks a regeneration, and the operator's only route is to edit the file (§9.6.1, R12). A scoped override (one class, one artifact, one invocation) is conceivable. | No override in v1. An override flag on a hard gate is how hard gates become suggestions. |
| Q10 | Is `90s` the right per-phase deadline? | Chosen as 1.5× the shipped single-phase `60s` (`internal/cli/cobra.go:629`) because a prepare-time call carries more context; no distribution has been measured. | `90s`, clamped to the remaining total. |
| Q11 | Should the retention surface be `feature intent-archive` (chosen) or a top-level `tpatch archive`? | The `feature` group already holds per-feature management verbs (`internal/cli/feature_deps.go:43-53`), but `intent-archive` is a long path to type. | `feature intent-archive`, consistent with `feature resource`. |
| Q12 | Should mutating `prepare` support BSD, `solaris` or `aix` through additional lock/statfs implementations? | The prepare-owned directory primitive and workspace-root filesystem preflight exist only on Linux/Darwin; admitting BSD without its own implementation and native CI would be an unsupported promise. | Refuse (§7.4.2, §12.6 D8). A future target needs its own primitive, workspace-filesystem proof and native CI. |
| Q13 | Should a known remote/user-space **workspace-root** filesystem be able to opt in? | The selected denylist protects the local authority from known unsupported classes; a flag would not make their semantics safe. | Refuse. A relaxation needs evidence for that class and a real-process test, not a flag. |

## 21. Alternatives considered (summary)

Beyond §8's six overwrite alternatives:

| Option | Verdict |
|---|---|
| A new `prepared` FeatureState | **Rejected.** WP-005 Agreed item 6 forbids it, and completeness is an artifact-level fact that `prepare --check` already reports. |
| A separate verb (`tpatch bundle`, `tpatch intent`) instead of extending `prepare` | **Rejected.** It would fork the vocabulary the accepted PRD, WP-005 and GH #10/#11 already share, and leave `prepare --check` orphaned from its own mutating half. |
| Redo-style recovery (complete the publication like `land`) | **Rejected.** Nothing in the publication is irreversible, so undo is strictly simpler and cannot publish from a pruned staging tree (§7.5, ADR-035 D5). |
| Taking the transaction lock in every command that writes a feature artifact | **Rejected.** Eleven shipped verbs would gain a lock, a deadlock surface and a new failure mode, to close a window that revalidation and per-entry CAS already detect (§7.4.4, §7.6.3). |
| Copy-on-write of the whole feature directory, then swap | **Rejected.** Directory swap is not atomic either, breaks any open descriptor, and would move audit-trail files (`patches/**`, `record.md`) that this command has no business touching. |
| Writing the archive to `.tpatch/local/` | **Rejected.** A recovery guarantee that vanishes on clone is not a guarantee (§9.1, ADR-035 D8). |
| Storing preimage **content** in the journal instead of the archive | **Rejected** for artifacts (unbounded gitignored growth, lost on clone); **adopted** for the two small machine-written metadata preimages, where the archive would be circular or category-wrong (§7.6). |
| `O_EXCL` lock file plus nonce/inode ownership (rev-0's model, and `land`'s) | **Rejected.** A file's existence is data, not liveness; CP1/CP2/CP8 and every `--manual` crash leave a lock with no journal to match, which permanently refuses the slug (§7.4.4). Replaced by the held-root kernel authority (§7.4.1). |
| Keeping the journal under the Git directory so `git clean` cannot remove it | **Rejected.** It does not survive a fresh clone either, `.git` is Git's namespace, worktree/submodule indirection complicates it, a non-Git workspace still needs a second location, and it would make the journal a hard Git dependency (§7.11.1). |
| Letting `--dry-run` simulate the post-recovery plan | **Rejected.** Deciding recovery's outcome without holding the lock and without performing it is exactly the guess §7.6.3 forbids; `--dry-run` refuses with `recovery-pending` instead (§6.4). |
| Keeping heuristic fallback for `--regenerate` (rev-0's behavior) | **Rejected.** One expired context could replace three hand-authored documents with boilerplate and exit 0. Recoverable is not the same as acceptable; the opt-in is `--allow-heuristic` (§11.3.2). |
| Archiving prior bytes without a redaction scan | **Rejected.** ADR-027 D3 makes redaction a precondition for every tpatch-owned writer, and a tracked archive is the worst possible place to silently retain a credential (§9.6.1). |
| Scrubbing secret-shaped content instead of refusing | **Rejected.** A scrubbed blob is not the bytes that were replaced, so the recovery guarantee would silently become false (§9.6.1). |
| An immutable archive with no removal route (rev-0's "normal and permanent") | **Rejected.** Immutable must mean "never modified in place", not "undeletable forever"; without §9.7 the design is a retention leak with a nice hash function. |
| Path-based `os.Rename` after rooted checks (rev-0's publication) | **Rejected.** `(*os.Root).Rename` prevents resolution outside the held root; it does not prove an ancestor did not redirect inside that root, which §7.7.2 discloses. |
| Bringing `FEATURES.md` into the publication set | **Rejected.** CAS on a derived index that eleven unlocked commands rewrite would turn ordinary concurrency into exit-5 aborts (§12.3.1). |
| Running automatic recovery before the `--abandon-transaction` branch (rev-4's order) | **Rejected.** It makes the escape unreachable for exactly the populations that need it: recovery either consumes the evidence the operator asked to preserve, or refuses exit 6 first — which is the state abandon exists to leave. The branch is now taken after the lock and before both the journal recovery and the pending-hash gate (§6.6, §7.8, §10.5). |
| Writing the journal and preimages with the shipped path-based `gitutil.DurableWriteFile` (rev-4) | **Rejected.** It is the right durability shape and the wrong resolution model: it takes a directory pathname and `os.CreateTemp`s inside it, so a local-lane write would sit outside D2's rooted guarantee while carrying the undo evidence for a rooted publication. The shape is reimplemented on the rooted primitives instead (§7.5, §7.7.1). |
| Exact-content rehydration as a repair for a dangling retained reference (rev-4) | **Rejected as unreachable.** X11 refuses every ordinary archive and canonical mutation while the dangling reference exists, so no run can ever reach the point of writing the replacement blob. The single shipped repair is the confirmed `purge --blob <hash> --yes` (§9.3.1, §9.7.3). |
| Claiming `--dry-run` reproduces every real-run refusal | **Rejected.** It cannot, by construction: it deliberately runs no mutating platform, filesystem, Git, lock or recovery gate. rev-5 enumerates the reproduced and non-evaluated populations and reports `execution_preflight: not_evaluated` instead of an unsupportable equivalence (§6.4). |
| Reporting a purge that failed after its first mutation as exit 3 (rev-4) | **Rejected.** Exit 3's contract is "wrote nothing", and by then the per-hash machine has written. Widening exit 3 would make the one code a harness can trust for zero-write meaningless; a distinct retryable `archive-purge-partial` at exit 5 carries it instead (§9.7.2, §10.4). |
| A non-creating doctor flock probe (rev-4) | **Rejected.** A real `LOCK_EX\|LOCK_NB` on the real authority makes a concurrent mutator refuse `transaction-in-progress`, and two concurrent doctors diagnose each other. A health check that can fail the thing it checks is a defect at any window size (§12.5). |
| Scrubbing `GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM` from the ignore gate | **Rejected.** The gate's question is whether the lane is ignored *for this operator*, and the answer legitimately depends on their `core.excludesFile`. Suppressing it would manufacture false `local-lane-not-ignored` refusals. The scrub covers repository selection and per-invocation inline config injection only, and §7.13 states that it is not a sandbox. |
| Applying prepare's environment scrub to the existing `gitutil`/`rescap` callers as a side effect of the refactor | **Rejected for this cluster.** It is a real behavior change to shipped commands with existing goldens, and it would be smuggled in under a seam move. The refactor keeps explicit compatibility wrappers; a future wave may authorize and test the change deliberately (§7.13, §17.2). |
| A byte scan proving the token `generator` appears in no `.tpatch/` file | **Rejected as infeasible.** Canonical artifacts are prose that may legitimately contain the word. The guard walks declared schema keys and enumerated persistence sinks instead (§10.2, §13.4). |
| Continuing into the requested operation after a successful recovery (rev-5) | **Rejected.** Recovery writes — it restores canonical bytes and removes the journal — and the gates below it can return exit 2 or exit 3, whose contract is "wrote nothing". Qualifying exit 3 as "zero-write except after recovery" makes the one code a harness can rely on conditional on state the harness cannot observe. Recovery is terminal at exit 0 `recovered` instead, and the operator re-runs (§7.8, §7.11, §10.4). |
| A `--continue-after-recovery` opt-in that restores the fall-through | **Rejected.** It would make an exit code's meaning depend on a flag, which is the same defect one command-line argument further away. There is no such flag, key or environment variable (§7.11). |
| Letting `intent-archive purge` recover a pending prepare journal (rev-5) | **Rejected.** A pending journal is an interrupted *publication* whose undo evidence binds canonical artifacts, `status.json` and the index together; recovering it from a retention verb would undo a publication the operator did not ask about. `purge` refuses `recovery-pending` without decoding, moving or consuming the journal, and recovers only pending *purge* state (§9.7, ADR-035 D16). |
| Requiring the local-lane Git gate before `--abandon-transaction` (rev-5) | **Rejected.** Abandon writes no new content: it renames existing lane entries one directory deeper in the same lane, so the gate's privacy question is already answered. Requiring a working `git` would make an absent or broken Git a permanent block on the only escape from exit 6 (§6.6, §7.13). |
| Leaving "exit 6 is never terminal" unqualified when the platform, filesystem or `flock` denies abandon | **Rejected.** The claim would be false in exactly the environments where an operator most needs it. The three environmental refusals name the repo-relative lane and a last-resort manual removal, and every recoverability claim in this cluster carries that qualification (§6.6, §10.4). |
| Echoing the operator's command line verbatim in a retry, including an inherited `--path` (rev-5) | **Rejected.** `--path` is absolute in ordinary use, and §10.6/§10.7 forbid absolute paths in reports. Every retry is the equivalent command without root-selection argv plus `retry_cwd: "workspace-root"` (§9.7.2). |
| Declaring `--yes` mutually exclusive with the default mode in cobra (rev-5) | **Rejected as unimplementable.** The default mode is the *absence* of every mode flag, and `MarkFlagsMutuallyExclusive` takes flag names. The command's own preflight rejects `--yes` outside `abandon` with one fixed literal at §10.5 step 1a instead (§5.2). |
| Classifying the root filesystem with `syscall.Fstatfs(int(f.Fd()), …)` (rev-5) | **Rejected.** `Fd()` yields a bare integer with no reference to the `*os.File`, reintroducing the naked-descriptor hazard the authority explicitly rejects for the `flock` on the very same file. `fstatfs` runs inside a `SyscallConn().Control` callback like every other authority syscall (§7.4.2). |
| Routing `archive-purge-evidence-divergent` to `--abandon-transaction` (rev-6) | **Rejected as a dead end.** Abandon touches no archive index and no blob (§6.6 rules 8 and 10), so it would refuse `no-pending-transaction` and leave the operator with two refusals and no route. The population gets its own repo-relative procedure — preserve, remove the divergent managed blob path (or restore an index that stopped decoding), rerun the sanitized purge whose pending+absent recovery terminally tombstones — and the message is forbidden from naming abandon (§9.7.2, §10.4). |
| Deleting the whole archive `index.json` as the repair for an index that stopped strict-decoding | **Rejected.** It would discard every generation record in the archive to resolve one hash. The route is to restore the file from the operator's own version control or backup and rerun; tpatch will not synthesize an index it cannot decode (§9.7.2). |
| Keeping “exactly three conditions can stop abandon” (rev-6) | **Rejected as a count rather than a proof.** Parse, slug, workspace and read-platform stops all precede the mutating gate, and rev-6's §10.5 step 6 additionally let an absent, malformed or unreadable feature refuse the escape. rev-7 replaces the count with a total gate table and removes step 6 from the mode (§6.6, §10.5). |
| Offering the last-resort lane `rm -rf` on a `transaction-in-progress` refusal | **Rejected.** The evidence may be the live undo journal of a publication a sibling process is executing right now, and the holder's identity is unknowable (§12.5). Deleting it under a live holder would destroy the undo evidence of a running transaction, so contention's only route is wait and retry (§6.6's gate table row 7). |
| Keeping PIB-283's “every mutating mode, including `--abandon-transaction`” lane-gate row (rev-6) | **Rejected as self-contradictory.** rev-6 made abandon bypass G1–G4 and then left a source-scan row asserting the gate runs in that mode. The row is amended to the normal mutating modes and explicitly excludes abandon, dry-run, check, list and purge (§7.13, §18.1). |
| Running G1 for `intent-archive purge` (rev-6's “per mutating invocation or purge”) | **Rejected.** Purge's entire write set is the tracked archive; it creates no staging tree and writes no byte under `.tpatch/local/`, so the lane gate has no subject there. Purge executes zero Git processes in both forms (§7.13, §9.7). |
| Telling every partial purge that one run recovers and a second completes (rev-6) | **Rejected as false for two of three branches.** A failure exactly between hashes leaves no pending marker, and `--orphans` rewrites no index at all; both finish in a single run and neither ever emits `recovered`. The closed `resume` field names the branch and the prose is written per branch (§9.7.2, §10.4). |
| Letting a `purge` preview perform the pending-hash recovery it reports | **Rejected.** Preview is defined by writing nothing and taking no lock; recovering would make the confirmation flag decorative on the one command whose whole gate is that flag. The preview states that recovery is required, lists the hash and plan, and names the `--yes` rerun (§9.7). |
| Leaving `--allow-heuristic` off the default-mode grammar line while §5.2 declares it legal there (rev-6) | **Rejected.** rev-6 fixed the prose and left the grammar block disagreeing with it, which is the same defect one paragraph away. The flag is on both lines it is legal on, with its no-op/advisory semantics unchanged (§5.1). |
| Filing a tombstone whose named blob is present under `archive-purge-evidence-divergent` (rev-7) | **Rejected as unexecutable.** That population's escape names a pending hash, removes the divergent managed blob and reruns the purge so the pending+absent case finalizes the tombstone. A tombstoned reference has no pending record: there is no hash to name, no purge transaction in flight, and nothing for the rerun to finalize. The state is exit-3 `archive-index-storage-inconsistent`, zero-write on discovery, and **which** repair it carries is decided from the hash's global liveness — `purge <slug> --orphans --yes` when the hash is unreferenced everywhere, and the confirmed `purge <slug> --blob <h> --yes` when another reference still retains it, `--orphans` being forbidden from touching a live blob (§9.3, §9.3.1, §9.7.3). |
| Letting a mutating `prepare` run `RecoverPendingPurge` (rev-6, rev-7) | **Rejected as a second owner for one transaction.** Both commands terminated after recovering, so exit 3's zero-write promise was safe; the cost was elsewhere. `archive-purge-evidence-divergent` and `archive-purge-partial` are *discovered by* the per-hash machine, so a second owner meant `prepare` could emit codes whose entire remediation, exit-window scoping and route belong to `purge`. The machine now has one call site, and `prepare` refuses exit 3 `recovery-pending` naming the purge command with `retry_cwd` (§7.8 step 5, §7.11, §10.4.1). |
| Duplicating §9.7.2's archive remediation into `prepare`'s output so `prepare` could keep recovering | **Rejected.** It keeps two owners and adds a second copy of every archive route sentence that must then stay in agreement with the first — the failure mode this cluster has already paid for twice in the exit-6 partition. One refusal that hands over one command is smaller and cannot drift (§7.8's "Why step 5 refuses"). |
| Exempting `--orphans` from the pending-hash recovery and from the pending-purge preview (rev-7) | **Rejected.** The orphan set is derived from the index's live-reference set, and a removal-pending reference is one whose liveness is mid-decision, so an orphan scan against a pending index computes its answer from state that is about to change. `--orphans` takes the same terminal recovery first and the same `recovery-required` preview as every other selector (§9.7, §9.7.1). |
| Reporting a widened `--all --yes` retry from any pending-purge preview | **Rejected.** The retry line is the one line an operator is most likely to copy and run unread, so it must be their command with `--yes` appended and nothing else — never widened for convenience, never narrowed to the pending hash. The recovery `--yes` performs is identical on every selector, so preservation costs nothing (§9.7, PIB-528). |
| Carrying the pending-purge preview on the existing `planned` or `refused` outcome token | **Rejected.** `planned` belongs to `--dry-run` and implies a plan the command would execute; `refused` implies a non-zero exit and a `refusal` object. The preview is neither: it exits 0, plans nothing and refuses nothing. It gets its own closed token `recovery-required` and one closed object (§10.2). |
| Listing §10.5 step 1a's `--yes` validation, the `--check` handoff and the `--dry-run` branch as pre-abandon gate rows (rev-7) | **Rejected as rows no fixture can reach.** All three are excluded by this mode's own grammar, and rev-7's own cell said so while still listing one of them. The table is defined over the argv that request a *true* abandon or fail parsing while naming the flag, and PIB-511 fails a row that no fixture in that domain can produce as well as a stop that no row covers (§6.6). |
| Defining the pre-abandon gate table's domain as "every argv in which `--abandon-transaction` appears" (rev-9) | **Rejected as over-reaching in the other direction.** `--abandon-transaction=false` is a legal false boolean that selects no mode: it runs `generate`'s contract end to end (§5.2), so a table of abandon's stops would be claiming rows over an invocation that never enters the mode. The domain is every argv that requests a *true* abandon, plus every argv that names the flag and stops in the parse layer — which keeps row 1 reachable and keeps `--check --abandon-transaction=false`, a presence-keyed mutex error, inside it (§6.6, PIB-511, PIB-537). |
| Letting the per-hash purge recovery act on the pending references alone (rev-9) | **Rejected as a way to manufacture the exact defect the design prevents.** With `G1` retaining `h` and `G2`'s reference to `h` removal-pending, rev-9's recovery removed `h.blob` and left `G1` retained — a dangling reference created by tpatch itself, which rev-9 then documented as a composition whose repair was the confirmed purge. A recovery that repairs the archive must not be a producer of the states it repairs. A pending reference now makes the transaction the **global owner** of the hash: the recovery claims every reference to `h` before removing anything, revalidates immediately before the removal, and never removes while a retained reference survives (§9.3, §9.7.2). |
| Refusing the mixed pending/retained index at X11 instead of claiming it | **Rejected as a brick.** X11 refuses ordinary mutation, so a refusal here would stop the one command that can advance the transaction — the operator would hold an index that no command may act on and whose only documented repair is the command that just refused. Normalizing under the owning transaction resolves the state instead of freezing it, and the end state (every reference to `h` tombstoned, no blob) is the one the pending record already committed to. |
| Un-tombstoning the already-tombstoned references instead of claiming them into the pending set | **Rejected.** It is the same reversal §9.3.1 subcase B rejects, one layer down: it would adopt bytes back into a reference the operator purged, on the strength of a transaction that was already removing them. The claim moves them *through* pending to tombstoned, which is where they already were, so the claim changes no reference's final state (§9.3). |
| Retrying the global claim in-process until no new same-hash reference appears | **Rejected as unbounded.** The workspace authority excludes tpatch mutators only, so an external writer inserting references can defeat any fixed retry count and starve any unbounded loop, while holding the workspace lock the whole time. A revalidation failure is reported as `archive-purge-partial` and the operator's retry re-derives the claim — a loop with a human at the top of it, which terminates and is observable (§9.7.2). |
| Validating X11 only over the references the requested selector touches | **Rejected as partial repair of a lying index.** It would let `purge --orphans --yes` delete residue while a mixed tombstone/live-reference hash elsewhere in the same index went unmentioned, or let `--blob h₃ --yes` complete beside a dangling `h₄` — leaving an archive that is *partly* repaired and whose next observation differs from the one just acted on. The scan is global before every archive mutation, the refusal is zero-write, and each admitted repair is admitted only for the exact inconsistency it repairs after global classification (§9.3.1, §9.7.1, PIB-542). |
| Treating a present but hash-wrong blob under a **retained** reference as dangling, or as an orphan | **Rejected on both readings.** It is not dangling: the file exists, and tombstoning a live reference against bytes tpatch never validated would destroy evidence the operator has not seen. It is not an orphan: the hash is live, so `--orphans` may not select it under any circumstances. It is `archive-blob-corrupt` at exit 3, zero-write, with one repo-relative route — the type-total removal of the managed blob path, then the confirmed `--blob <h> --yes` that is by then the ordinary dangling repair, or restore the exact correct blob and retry (§9.3.1, §9.7.3). |
| Exempting already-tombstoned same-hash references from the global claim (rev-10) | **Rejected as a deadlock, not an optimization.** rev-10's step 2 gates the removal on *every* reference to `h` being removal-pending while two neighbouring sections called an already-tombstoned reference a "deterministic no-op". An index holding one tombstoned and one retained reference to `h` therefore had no reachable removal at all: the claim would decline to touch the tombstone, the gate would see a non-pending reference forever, and the hash would stay owned with no command able to advance it. The exemption is withdrawn; the claim is total over the hash and the tombstoned reference passes through pending back to tombstoned, changing no final state (§9.3, §9.7.2, PIB-532, PIB-545). |
| Ordering the whole-index X11 scan **before** the pending-purge recovery | **Rejected as a validation gate bricking the transaction it protects.** rev-10 said the global scan preceded *every* archive mutation and separately said the compound `--yes` run finalizes `h₁` while `h₂` is inconsistent; read together, the scan refused the run and `h₁` stayed pending forever, with the only named repair being the command that had just refused. Completing an already-owned transaction is the one explicit exception: the recovery runs first, is terminal, never proceeds into the selector, and the operator's rerun is an ordinary new-selector invocation fully governed by the global scan (§9.3.1, §9.7.2, PIB-546). |
| Admitting a repair only when its observation is the **sole** inconsistency in the index (rev-10) | **Rejected as a brick with two instances.** Two unreferenced residues each disqualified the other, so `--orphans --yes` was never admitted and the archive had no shipped repair; the same held for two dangling hashes and two mixed hashes. Admission is per repair **class** with full coverage: one class present, and a confirmed selection covering every instance of it, is admitted and repairs them all. Mixed classes remain a zero-write refusal rendering each class's own route (§9.3.1, §9.7.2, PIB-548, PIB-549). |
| Keeping the `cp` preservation step and plain `rm` in the corrupt/divergent procedures (rev-10) | **Rejected as unexecutable on its own fixtures.** The populations include a directory, a symlink, a FIFO and a device node at the managed blob path. `cp` without `-R` and `rm` without `-r` both fail on a directory; `cp` on a symlink copies the target rather than the link. One type-total `rm -rf --` against the exact validated path, under an explicit destructive warning, is printed instead, and an operator who wants the object is told to stop and use kind-appropriate tooling rather than being handed one command that is right for one kind out of five (§9.3.1, §9.7.2, §10.7, PIB-547). |
| Claiming the revalidate→unlink window is closed by re-checking the file's hash immediately before removal | **Rejected as unimplementable, and therefore not claimed.** Neither supported platform offers an unlink conditioned on content, so between the pre-removal revalidation and the syscall an external writer can replace the object and have the replacement removed. The window is disclosed as a residual beside the post-CAS rename race rather than closed in prose, and PIB-550 fails any shipped string or document sentence that says otherwise (§9.7.2, §7.1). |
| Letting `list` or `doctor` stop at the first exit-3 observation, or report only the worst one | **Rejected.** An operator repairing an archive needs the whole inventory in one pass; reporting one observation at a time turns an *n*-inconsistency archive into *n* discover-fix cycles, and each cycle's refusal looks identical to the last. Both surfaces render every safe observation with its own repair and take the highest exit, so the exit code answers "is anything here untrustworthy" while the body answers "what, exactly" (§9.7, §12.5, PIB-541). |
| Leaving the one retry heading's indentation unspecified (rev-9) | **Rejected.** rev-9 promised the sentence byte-for-byte and then indented it differently in different worked examples, so two emitters could satisfy the promise and still disagree on the bytes at the start of the line, and a harness matching the line would need each caller's nesting depth. The heading is a column-0 literal with no leading or trailing whitespace, alone on its line, and PIB-498's sensitivity set includes an indented and a trailing-space variant (§9.7.2). |

## 22. Claims-audit appendix

**176 claims (`C1`…`C176`)** record every load-bearing claim this PRD makes
about **current** behavior, with a
`file:line` anchor. A reviewer should spot-check that each anchor lands within
±5 lines of the cited construct at dispatch/base `c394117` (reviewed tip
`a9ad7c0`). Claims corrected in rev-1
are marked †; claims added in rev-1 start at C143, claims added in rev-5 start
at C166, and the claim added in rev-6 is C176. None of rev-7, rev-8, rev-9,
rev-10 or rev-11
adds a claim: none makes a new assertion about current shipped behavior, only
about this cluster's own contract, so the count stays at **176**.

| # | Claim | Anchor |
|---|---|---|
| C1 | `RunAnalysis` writes the sidecar `artifacts/analysis.json` before anything else | `internal/workflow/workflow.go:90` |
| C2 | `RunAnalysis` then writes `analysis.md` | `internal/workflow/workflow.go:96` |
| C3 | `RunAnalysis` then mutates feature state to `analyzed` | `internal/workflow/workflow.go:103` |
| C4 | `RunDefine` writes `spec.md` | `internal/workflow/workflow.go:151` |
| C5 | `RunDefine` marks `defined` with a fixed notes string | `internal/workflow/workflow.go:155` |
| C6 | `RunExplore` writes `exploration.md` | `internal/workflow/workflow.go:196` |
| C7 | `RunExplore` also marks `defined` | `internal/workflow/workflow.go:200` |
| C8 | `RunAnalysis` writes a raw provider response into `artifacts/` on the fallback path | `internal/workflow/workflow.go:72` |
| C9 | …and again on the parse-failure path | `internal/workflow/workflow.go:80` |
| C10 | `RunAnalysis` falls back to the heuristic generator on provider error | `internal/workflow/workflow.go:67-69` |
| C11 | `RunDefine` falls back to the heuristic generator | `internal/workflow/workflow.go:141-143` |
| C12 | `RunExplore` falls back to the heuristic generator | `internal/workflow/workflow.go:186-188` |
| C13 | Heuristic mode is recorded in the sidecar as `heuristic_mode` | `internal/workflow/workflow.go:25` |
| C14 | …and set by the heuristic analysis constructor | `internal/workflow/workflow.go:208` |
| C15 | `RunDefine` reads `analysis.json` and `analysis.md` as context | `internal/workflow/workflow.go:117-123` |
| C16 | `RunExplore` reads `analysis.md` and `spec.md` as context | `internal/workflow/workflow.go:165-166` |
| C17 | Analysis requests are capped at 4096 output tokens | `internal/workflow/workflow.go:58` |
| C18 | Define requests are capped at 4096 output tokens | `internal/workflow/workflow.go:132` |
| C19 | Explore requests are capped at 4096 output tokens | `internal/workflow/workflow.go:177` |
| C20 | The analysis validator requires a JSON **object** | `internal/workflow/workflow.go:62` |
| C21 | `captureFileTree` is a pure read used for generation context | `internal/workflow/workflow.go:262-266` |
| C22 | `readGuidanceFiles` is a pure read used for generation context | `internal/workflow/workflow.go:293-304` |
| C23 | `renderAnalysisMD` is already pure | `internal/workflow/workflow.go:351-401` |
| C24 | `GenerateWithRetry` writes `raw-<prefix>-response-<n>.txt` when a `Store` is supplied | `internal/workflow/retry.go:105-109` |
| C25 | `GenerateWithRetry` retries with a corrective prompt and returns the last response | `internal/workflow/retry.go:94-131` |
| C26 | `WithDisableRetry` forces a single attempt | `internal/workflow/retry.go:47-58` |
| C27 | …consumed as `attempts = 1` | `internal/workflow/retry.go:83-85` |
| C28 | `--manual` validates only that the artifact exists (JSON validation is implement-only) | `internal/store/manual.go:56-66` |
| C29 | The `--manual` phase map is the single source of truth for four phases | `internal/store/manual.go:26-32` |
| C30 | `AdvanceStateManually` writes a free-text note and marks state | `internal/store/manual.go:78-80` |
| C31 | `WriteFeatureFile` path-checks then calls `writeFile` | `internal/store/store.go:443-449` |
| C32 | `writeFile` is `os.WriteFile` — a truncating in-place write | `internal/store/store.go:918-923` |
| C33 | `writeFileAtomic` preserves the existing file's permission bits | `internal/store/store.go:871-876` |
| C34 | `writeFileAtomicWithRename` is temp→chmod→write→sync→close→rename→dir-sync | `internal/store/store.go:878-917` |
| C35 | The temp file is created in the destination directory | `internal/store/store.go:884-886` |
| C36 | `SaveFeatureStatus` writes `status.json` atomically and best-effort refreshes `FEATURES.md`, swallowing the refresh error because `status.json` is the source of truth | `internal/store/store.go:363-377` |
| C37 | `SaveFeatureStatus` stamps `updated_at` when empty | `internal/store/store.go:369-371` |
| C38 | `MarkFeatureState` overwrites `Notes` on every transition | `internal/store/store.go:380-393` |
| C39 | `MarkFeatureState` bumps `updated_at` | `internal/store/store.go:388` |
| C40 | Config stores an auth **env-var name**, not a secret | `internal/store/store.go:72` |
| C41 | `MkdirAllAndSyncChain` fsyncs the whole chain unconditionally | `internal/store/fsdurable.go:41-52` |
| C42 | …and the rationale is crash-durability of already-existing directories | `internal/store/fsdurable.go:1-10` |
| C43 | `SyncDir` opens and fsyncs a directory | `internal/store/fsdurable.go:22-33` |
| C44 | `RandomHex12` is the per-invocation scratch suffix convention | `internal/store/fsdurable.go:96-103` |
| C45 | The twelve `FeatureState` values | `internal/store/types.go:9-38` |
| C46 | `defined` is an existing state | `internal/store/types.go:11` |
| C47 | `ValidFeatureState` enumerates them in one switch | `internal/store/types.go:40-47` |
| C48 | `upstream_merged` asserts an upstream implementation exists | `internal/store/types.go:26-31` |
| C49 | `FeatureStatus` carries `state`, `last_command`, `updated_at`, `notes` | `internal/store/types.go:206-216` |
| C50 | `Verify` is written only by `verify` and `amend` | `internal/store/types.go:236-251` |
| C51 | `RejectableStates` is pre-implementation only | `internal/store/status.go:139-152` |
| C52 | A reopen restores `PriorState` | `internal/store/status.go:130-133` |
| C53 | The land journal exists because `git commit` is irreversible | `internal/cli/land_journal.go:11-23` |
| C54 | The land journal is versioned and refuses other versions | `internal/cli/land_journal.go:56-58` |
| C55 | It lives under the gitignored `.tpatch/local/` root | `internal/cli/land_journal.go:60-62` |
| C56 † | `landJournalFileState` is `(exists, sha256, mode)` with a `matches` comparison — it carries **no size**; rev-0 claimed it did, and §7.6.1 now states the size field as this design's deliberate extension | `internal/cli/land_journal.go:65-79` |
| C57 | Its `Phase` field is explicitly advisory | `internal/cli/land_journal.go:110-111` |
| C58 | The journal records a wall-clock `created_at` | `internal/cli/land_journal.go:109` |
| C59 | **`land`'s** journal is written through `gitutil.DurableWriteFile` | `internal/cli/land_journal.go:310-326` |
| C60 | The journal decoder is strict (unknown fields refused) | `internal/cli/land_journal.go:348-380` |
| C61 | Recovery classifies the live state as preimage / postimage / divergent | `internal/cli/land_journal.go:418-444` |
| C62 | Recovery runs before the caller mutates anything | `internal/cli/land_journal.go:445-482` |
| C63 † | `land`'s recovery lock is `O_CREATE\|O_EXCL` with a nonce, fsynced — cited in rev-1 only as the **rejected** alternative (§7.4.4), not as this design's lock | `internal/cli/land_journal.go:629-648` |
| C64 | Lock release removes the file and fsyncs the directory | `internal/cli/land_journal.go:650-662` |
| C65 † | `removeOwnedLandLock` removes a stale lock only when the nonce **and**, where the platform exposes one, the recorded inode match — the discipline rev-1 replaces with a kernel lock | `internal/cli/land_journal.go:675-698` |
| C66 | `DurableWriteFile` is temp→write→fsync→close→rename | `internal/gitutil/index_snapshot.go:455-500` |
| C67 | The resource capture tree is an unordered content-addressed set plus one pointer | `internal/store/resource_publish.go:1-9` |
| C68 † | Batch IDs are derived at exactly one point (a swappable deriver seam) | `internal/store/resource_publish.go:131-143` |
| C69 † | `ComputeBatchID` hashes the canonical body | `internal/store/resource_publish.go:155-167` |
| C70 | An already-identical batch file is an idempotent re-publish, never rewritten | `internal/store/resource_publish.go:240-246` |
| C71 † | Named publication refusals include collision and corruption | `internal/store/resource_publish.go:198-202` |
| C72 † | Publication is "write immutable content, then rewrite the pointer", and an existing file is compared **semantically** before being reused | `internal/store/resource_publish.go:230-285` |
| C73 | Batch decoding is strict: unknown fields, trailing content and null arrays refused | `internal/store/resource_publish.go:305-328` |
| C74 | No Go map may reach a wire format | `internal/store/canonjson.go:11-17` |
| C75 | The root printer emits `error: %v` for every non-nil `RunE` error | `internal/cli/cobra.go:33-39` |
| C76 | `exitCodeFor` maps typed errors to their code and everything else to 1 | `internal/cli/cobra.go:43-52` |
| C77 | The root sets `SilenceUsage` and `SilenceErrors` | `internal/cli/cobra.go:56-62` |
| C78 | `--path` is a root persistent string flag | `internal/cli/cobra.go:66` |
| C79 | `openStoreFromCmd` resolves `--path` then `FindProjectRoot` | `internal/cli/cobra.go:3782-3793` |
| C80 | The single-phase commands use the **probing** provider loader | `internal/cli/cobra.go:609-612` |
| C81 | They print progress to stdout | `internal/cli/cobra.go:621-626` |
| C82 | Their timeout default is 60s | `internal/cli/cobra.go:629` |
| C83 | `--no-retry` is a shipped phase flag | `internal/cli/cobra.go:630` |
| C84 | `--manual` and `--skip-llm` are installed by one shared helper | `internal/cli/cobra.go:3410-3414` |
| C85 | `runManualPhase` is the single manual entry point for four phases | `internal/cli/cobra.go:3429-3437` |
| C86 | `isTpatchUntracked` detects an untracked `.tpatch/` | `internal/cli/cobra.go:3405-3407` |
| C87 | `cycle` is a sequential pipeline through record | `internal/cli/phase2.go:26-32` |
| C88 | `cycle` calls the incremental phase functions in order | `internal/cli/phase2.go:62-96` |
| C89 | `cycle` asserts state after each phase | `internal/cli/phase2.go:69-71` |
| C90 | `cycle` uses the **non-probing** provider loader | `internal/cli/phase2.go:55` |
| C91 | `cycle --interactive` is the repo's only interactive prompt precedent | `internal/cli/phase2.go:50` |
| C92 | `cycle --skip-execute` stops before recipe execution | `internal/cli/phase2.go:122-126` |
| C93 | `next` emits a single harness task and supports `--format harness-json` | `internal/cli/phase2.go:381-384` |
| C94 | `next` infers the `defined` sub-state from `exploration.md` presence | `internal/cli/phase2.go:437-446` |
| C95 | `refuseIfUnappliedState` refuses with the shipped state-refusal code | `internal/cli/feature_unapply.go:464-473` |
| C96 | The repo binds exit 2 to validation and exit 3 to state refusal | `internal/cli/reject.go:45-48` |
| C97 | …surfaced through `ExitCodeError` | `internal/cli/reject.go:68-70` |
| C98 | `doctor` runs a registry of checks D1…D8 | `internal/workflow/doctor.go:226-237` |
| C99 | `land` sweeps everything dirty under the feature directory into its path set | `internal/cli/land.go:723-725` |
| C100 | The `.tpatch/local/` prefix is the gitignored scratch root | `internal/rescap/scratch.go:34-37` |
| C101 | A local-lane ignore contract gate runs before any scratch write | `internal/rescap/scratch.go:47-62` |
| C102 | Skill parity is enforced by a required-command list | `assets/assets_test.go:14-53` |
| C103 | …and by verbatim required anchors including phase ordering and preflight | `assets/assets_test.go:62-73` |
| C104 | `SPEC.md` makes exit codes per-command contracts | `SPEC.md:137` |
| C105 | …and binds exit 3 to state-transition refusal for the reject family | `SPEC.md:148` |
| C106 | The Path B guide teaches authoring all three Markdown files then three `--manual` runs | `docs/path-b-operator-guide.md:61-73` |
| C107 | The phase → artifact → state contract table | `docs/agent-as-provider.md:40-45` |
| C108 | `status.json.notes` is currently presented as distinguishing Path B from provider output | `docs/agent-as-provider.md:47-54` |
| C109 | The feature layout marks `post-apply.patch` canonical and `patches/**` audit-only | `docs/feature-layout.md:19,34` |
| C110 | The recipe is written by the implement phase | `docs/feature-layout.md:85` |
| C111 | Capture batches are an unordered content-addressed set, not a chronology | `docs/feature-layout.md:98-100` |
| C112 | An orphaned batch left by a crash is normal and permanent | `docs/feature-layout.md:103-106` |
| C113 | WP-005 Agreed item 3: downstream SDD is encouraged, never enforced | `docs/whitepapers/WP-005-spec-driven-workflows.md:56-58` |
| C114 | WP-005 Agreed item 6: no new lifecycle state; exploration must not become mandatory | `docs/whitepapers/WP-005-spec-driven-workflows.md:71-74` |
| C115 | WP-005 Agreed item 7: the all-or-nothing publication unit and the complete-prior-or-complete-new rule | `docs/whitepapers/WP-005-spec-driven-workflows.md:75-81` |
| C116 | WP-005 Turn 3: prepare cannot call the incremental writers and claim atomicity | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:112-117` |
| C117 | WP-005 Turn 3: the required existing-primitives pre-flight | `docs/whitepapers/WP-005-spec-driven-workflows.turns.md:118-123` |
| C118 | The accepted PRD reserves plain `prepare` with exit 4 and a frozen refusal | `docs/prds/PRD-artifact-validation-and-provenance.md:356-382` |
| C119 | …and anticipates a future PRD registering the flags as an enumerated delta | `docs/prds/PRD-artifact-validation-and-provenance.md:396-403` |
| C120 | The accepted required set is the three Markdown artifacts | `docs/prds/PRD-artifact-validation-and-provenance.md:432-436` |
| C121 | The sidecar is optional and never affects readiness | `docs/prds/PRD-artifact-validation-and-provenance.md:437-441` |
| C122 | The closed nine-value state enum | `docs/prds/PRD-artifact-validation-and-provenance.md:1765-1773` |
| C123 | The frozen non-certification disclaimer | `docs/prds/PRD-artifact-validation-and-provenance.md:1796-1798` |
| C124 | The accepted exit-code table (0/1/2/3/4) | `docs/prds/PRD-artifact-validation-and-provenance.md:1981-1987` |
| C125 | The accepted stream-routing contract | `docs/prds/PRD-artifact-validation-and-provenance.md:2367-2419` |
| C126 | The accepted JSON schema and its no-map rule | `docs/prds/PRD-artifact-validation-and-provenance.md:2420-2576` |
| C127 | The accepted zero-mutation contract | `docs/prds/PRD-artifact-validation-and-provenance.md:2840-2866` |
| C128 | The forbidden provenance-inference sources | `docs/prds/PRD-artifact-validation-and-provenance.md:2904-2920` |
| C129 | The provenance ADR trigger fires only on selection of a persistent **provenance** representation | `docs/prds/PRD-artifact-validation-and-provenance.md:2966-2972` |
| C130 | ADR-034 D14 forbids citing ADR-034 as provenance precedent | `docs/prds/PRD-artifact-validation-and-provenance.md:2957-2961` |
| C131 | The accepted decision that `--manual` gates stay loose | `docs/prds/PRD-artifact-validation-and-provenance.md:2986-3015` |
| C132 | The accepted disqualifying assertion shapes | `docs/prds/PRD-artifact-validation-and-provenance.md:3531-3548` |
| C133 | The pre-change-golden prerequisite lesson | `docs/prds/PRD-artifact-validation-and-provenance.md:3515-3520` |
| C134 | The key-and-label scoping rule for forbidden-field guards | `docs/prds/PRD-artifact-validation-and-provenance.md:2554-2556,3626` |
| C135 | The guard-sensitivity requirement | `docs/prds/PRD-artifact-validation-and-provenance.md:3960-3991` |
| C136 | The accepted §20 list of what this PRD must answer | `docs/prds/PRD-artifact-validation-and-provenance.md:4016-4047` |
| C137 | The accepted slug grammar and its no-echo rule | `docs/prds/PRD-artifact-validation-and-provenance.md:696-772` |
| C138 | The accepted per-artifact classification ladder | `docs/prds/PRD-artifact-validation-and-provenance.md:1653-1782` |
| C139 | The accepted snapshot/instability semantics and their stated limits | `docs/prds/PRD-artifact-validation-and-provenance.md:1827-1917` |
| C140 | The accepted `--help` mitigation for the `apply --mode prepare` collision | `docs/prds/PRD-artifact-validation-and-provenance.md:344-355` |
| C141 | The accepted self-contained-remediation rule | `docs/prds/PRD-artifact-validation-and-provenance.md:374-381` |
| C142 | The accepted docs-update obligations, including the `agent-as-provider` correction | `docs/prds/PRD-artifact-validation-and-provenance.md:3372-3435` |
| C143 | `compareSemanticBody` is the shipped semantic-CAS precedent | `internal/store/resource_publish.go:358-399` |
| C144 | The shipped per-slug lock is a kernel `flock(2)` on an open file description, taken `LOCK_EX\|LOCK_NB` after a statfs preflight and a whole-chain fsync | `internal/rescap/lock_unix.go:63-88` |
| C145 | It is released by **closing the descriptor**, with no separate unlock step that could fail or leave a stale artifact | `internal/rescap/lock_unix.go:43-52` |
| C146 | Its rationale is explicitly that a kernel lock has none of the ABA/staleness problems of a PID-plus-rename protocol, and is released on SIGKILL, crash or power loss | `internal/rescap/lock_unix.go:6-11` |
| C147 | Unsupported build targets refuse unconditionally without creating or opening anything | `internal/rescap/lock_unsupported.go:29-32` |
| C148 | `LockSupported` is a build-tagged constant, true only on linux/darwin today | `internal/rescap/lock_unix.go:26`, `internal/rescap/lock_unsupported.go:15` |
| C149 | The CLI already runs every resource mutation under that lock | `internal/cli/feature_resource.go:101-116` |
| C150 | The local ignore contract **refuses when Git is unavailable** | `internal/workflow/session_ignore.go:138-145` |
| C151 | `IsGitAvailable` shells out to `git rev-parse --is-inside-work-tree` after a `LookPath` | `internal/gitutil/ignore.go:25-34` |
| C152 | `IsPathIgnored` shells out to `git check-ignore -q --no-index` | `internal/gitutil/ignore.go:59-78` |
| C153 | The untracked half shells out to `git --literal-pathspecs ls-files` | `internal/rescap/gitgate.go:104-115` |
| C154 | `redact.Scan` applies the closed six-class resource policy to in-memory bytes and returns sorted, deduplicated codes | `internal/redact/redact.go:81-90` |
| C155 | The six resource classes are private-key, connection-url, email-pii, credential-assignment, bearer-or-key-token, home-absolute-path | `internal/redact/redact.go:53-72` |
| C156 | `Scan` deliberately takes bytes and never a path, so "scan before write" is structural | `internal/redact/redact.go:18-21` |
| C157 | ADR-027 D3 makes redaction a write precondition for every tpatch-owned writer, and a hard failure for committed artifacts | `docs/adrs/ADR-027-capture-context-privacy-boundary.md:146-177` |
| C158 | `session purge` is dry-run-by-default with an explicit `--yes` gate and a required scope | `internal/cli/session.go:353-419` |
| C159 | …and its rev-1 lesson was that "neither selector" must not silently mean "all" | `internal/cli/session.go:369-375` |
| C160 | `tpatch feature` is the shipped per-feature management group | `internal/cli/feature_deps.go:43-53` |
| C161 | `feature resource remove` / `clear` are the shipped per-feature removal verbs | `internal/cli/feature_resource.go:655,699` |
| C162 | `RefreshFeaturesIndex` rebuilds the index from every feature's status | `internal/store/store.go:703-712` |
| C163 | The toolchain is pinned at Go 1.26.1, which provides `(*os.Root).Rename` | `go.mod:3` |
| C164 | The shipped atomic writer creates its temp with `os.CreateTemp` and renames path-based, so it is **not** a rooted writer | `internal/store/store.go:878-917` |
| C165 | `land`'s journal path containment is validated before use | `internal/cli/land_journal.go:383-415` |
| C166 | `gitutil.DurableWriteFile` creates its publish temp with `os.CreateTemp` inside a directory **pathname** it is handed, which is what makes it a path-based writer | `internal/gitutil/index_snapshot.go:455-456` |
| C167 | `rescap.IsIgnored` spawns `git check-ignore -q --no-index` directly with `exec.Command`, setting only `cmd.Dir`, with no environment scrub | `internal/rescap/gitgate.go:45-48` |
| C168 | `rescap.IsTracked` spawns `git --literal-pathspecs ls-files --error-unmatch` the same way | `internal/rescap/gitgate.go:76-78` |
| C169 | `gitgate.go` documents two deliberate invocation asymmetries — `check-ignore` without `--literal-pathspecs`, and the leading-colon `./` rule — which a central-executor refactor must preserve | `internal/rescap/gitgate.go:1-20` |
| C170 | `rescap.RunGit` is the shipped generic Git runner, also unscrubbed | `internal/rescap/gitgate.go:120-124` |
| C171 | Linux `Statfs_t.Type` is normalized to a single `uint32` because its width and signedness are architecture-dependent | `internal/rescap/statfs_linux.go:57-61` |
| C172 | The shipped Linux denied-family map names `nfs`, `cifs`, `smb2` and `fuse` by magic number | `internal/rescap/statfs_linux.go:49-54` |
| C173 | The shipped Darwin denied-name map is exactly `nfs`, `smbfs`, `webdav`, `osxfuse`, `macfuse` | `internal/rescap/statfs_darwin.go:28-34` |
| C174 | Darwin's `Fstypename` is a `[16]int8` array requiring a signed-to-unsigned conversion and a trim at the first NUL | `internal/rescap/statfs_darwin.go:39-49` |
| C175 | The shipped resource-capture filesystem preflight classifies by **path**-based `syscall.Statfs`, not by descriptor — the ordering this PRD deliberately does not copy (§7.4.2) | `internal/rescap/statfs_linux.go:80-84`, `internal/rescap/statfs_darwin.go:62-66` |
| C176 | The shipped `rescap` lock passes a bare `int(f.Fd())` to `syscall.Flock` — the naked-descriptor form this PRD's authority path deliberately does not copy, using `SyscallConn().Control` for `fstatfs`, `flock` and the unlock instead (§7.4.1, §7.4.2) | `internal/rescap/lock_unix.go:80` |
