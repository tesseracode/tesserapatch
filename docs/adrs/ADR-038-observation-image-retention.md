# ADR-038: Bounded in-memory observation image retention

**Status**: Proposed — implemented for GH #15 S2, pending review
**Date**: 2026-09-07
**Scope**: Internal `patchobs` capture and recorder ownership only
**Related**: ADR-036 D2, D5, D16; GH #15 S1/S2

## Context

ADR-036 requires derivation from an immutable, pre-write observation.
Observations retain both preimage and postimage bytes. S1 batches Git
processes, but its worktree reader and buffered `cat-file --batch` output
retain arbitrarily large bodies. The batch stdout buffer additionally
duplicates every Git body before copying it into the observation.

Dropping preimages after S2 derivation would make S3's planned simulation
depend on a new read, violating the same observation boundary. A cleanup
method alone would not protect against callers retaining observations.
Accepted ADR-036 is not amended by this internal resource policy.

## Decision

1. **32 MiB of distinct retained image backing arrays per observation.**
   The same budget covers all working-tree bodies and all Git bodies,
   across both references, while they are read and after they are folded
   into effects. It is not a per-file or per-side limit. Worktree paths
   consume the budget in byte-sorted order, followed by byte-sorted,
   deduplicated Git object IDs. Oversized bodies consume no budget;
   later smaller bodies and proven absence can still be observed.
   An empty present file consumes zero bytes and remains distinct from
   an unavailable or absent side.

2. **Enforce before allocation, not after capture.** Regular worktree
   files are size-checked before opening and checked again using the
   opened descriptor's size. A fixed-size allocation reads precisely
   that body; a one-byte EOF probe refuses growth instead of retaining
   a prefix. Short reads also refuse the side. Symlink sizes are checked
   before `Readlink` and the returned target length is checked before
   making the retained byte array. The operating system's bounded link
   target string is transient, not an unbounded file-content buffer.

   Git stdout is parsed directly from a pipe with a 4 KiB header buffer.
   Declared sizes are parsed as signed 64-bit integers and checked before
   allocating bodies. Non-blobs and oversized bodies are drained with
   32 KiB scratch space. There is no whole-batch stdout buffer or copied
   raw-body staging area. Malformed/truncated frames are not retained;
   an unresynchronizable stream terminates and reaps its child process.
   Fully read preceding frames remain usable.

3. **Fail closed using existing availability semantics.** A refused body
   supplies no bytes, hash, mode, presence claim, or observation claim.
   Its side has `observed: false`, with `preimage-unavailable` or
   `postimage-unavailable`. Human diagnostic text in the existing
   `EffectObservation.Contradictions` field identifies the side, body
   size, remaining capacity, and total budget. No new coverage field or
   wire reason is introduced. Classification continues through the
   unchanged S1 rules: an unavailable extant side cannot establish text;
   a binary stanza remains independent positive binary evidence.

4. **Retain captured bodies for the observation's entire lifetime.**
   Derivation and future simulation use the same immutable bytes;
   neither reopens live worktree files. No mutable `ReleaseBodies` API
   is added. Unreachable observations release their memory through Go's
   normal garbage collection. Keeping one observation alive cannot
   retain more than its image cap. The read maps and effects reference
   the same exact-capacity arrays rather than copying their bodies.

5. **Recorder ownership is independent.** `Emit` deep-copies mutable
   slices and artifact snapshots for a non-discard recorder, so recorder
   mutation cannot alter the producer's derivation input. Repeated Git
   objects retain one shared immutable array inside each copy; cloning
   cannot turn repeated effect references into an unbounded multiplier.
   Producer and recorder copies have separate arrays: at most 32 MiB
   of images each, plus bounded streaming scratch during capture.
   The default discard recorder does not clone. Callers must continue
   treating an observation's exported slices as immutable.

   `Input.ParentCreatedPaths` is also copied and sorted during capture
   and independently copied at the recorder boundary. It is the
   producer's pre-captured exclusion set for targets whose required
   preimage depends on parent-created bytes, not an ownership or history
   claim. It never authorizes parent-body reuse or adds a live reread to
   derivation; producers discover these exclusions before capture.

6. **Preserve process batching and local-only reads.** This changes
   `cat-file --batch` output handling, not the process budget: one
   invocation across both references; no per-effect Git process,
   network fetch, new dependency, temporary body artifact, or new Git
   version requirement. Worktree reads remain subprocess-free.

The cap is deliberately **image storage**, not a total-process memory
limit. Already supplied canonical patch bytes, bound-artifact snapshots,
effect/path metadata, Git's own implementation buffers, and any derived
recipe are outside this budget. The number of producer events a custom
recorder chooses to archive is also outside the per-observation contract;
retaining arbitrarily many observations is not a bounded event archive.
Neither a total-process cap nor an archive is introduced under S2.

## Alternatives considered

- **Keep every image until collection, without a cap:** rejected because
  caller lifetimes and one huge file can retain arbitrarily much memory.
- **Free only preimage bodies immediately after generation:** rejected
  because S3 needs the same preimages for simulation.
- **Per-file cap or post-read truncation:** rejected because many small
  bodies remain unbounded, and truncation cannot establish exact images.
- **Temporary body files:** rejected because D2 retains source bodies in
  memory only and introduces no new artifact/privacy lifecycle.
- **Read Git once per accepted object:** rejected because it loses S1's
  reference-batched process count.
- **Clone every effect body independently:** rejected because repeated
  Git object IDs can multiply retained bytes without adding information.

## Verification

`internal/patchobs/retention_s2_test.go` supplies `TestRGAS2*` behavioral
fixtures for exact/over/zero and cumulative boundaries, refusal before any
read of a huge declared body, bounded streaming of a synthetic 64 MiB
blob, short/growing files, malformed frame boundaries, honest unavailable
flags and diagnostics, continued capture of later smaller files, real Git
process counts and deduplication, the default limit against a sparse
oversized file, late worktree mutation, and recorder alias isolation.
Each resource boundary has a positive fixture and a rejecting fixture;
the streaming fixture measures both read-request size and allocation.

No Go validation was run by the implementer. The coordinating agent owns
the serial validation sequence and its required memory/load idle gate.
