//go:build (linux && !android) || (darwin && !ios)

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"syscall"
	"testing"
	"unicode"
	_ "unsafe"

	"github.com/tesseracode/tesserapatch/internal/intentlock"
	"github.com/tesseracode/tesserapatch/internal/provider"
	"github.com/tesseracode/tesserapatch/internal/store"
)

//go:linkname s7ARFailPurgeBetweenHashes github.com/tesseracode/tesserapatch/internal/store.failPurgeBetweenHashes
var s7ARFailPurgeBetweenHashes func() error

//go:linkname s7ARFailOrphanRemoveAfterFirst github.com/tesseracode/tesserapatch/internal/store.failOrphanRemoveAfterFirst
var s7ARFailOrphanRemoveAfterFirst func() error

type s7ARGitProcessRequest struct {
	repoRoot      string
	args          []string
	env           []string
	captureStdout bool
	captureStderr bool
}

type s7ARGitProcessResult struct {
	stdout   string
	stderr   string
	exitCode int
	err      error
}

//go:linkname s7ARRunGitProcess github.com/tesseracode/tesserapatch/internal/gitutil.runGitProcess
var s7ARRunGitProcess func(s7ARGitProcessRequest) s7ARGitProcessResult

type s7ARDivergenceFixture struct {
	root       string
	slug       string
	hash       string
	blobRel    string
	indexRel   string
	safeHash   string
	targetPath string
}

func TestS7ARDivergenceContracts(t *testing.T) {
	t.Run("PIB-506", func(t *testing.T) {
		for _, kind := range []string{"regular", "symlink", "directory", "fifo", "device-seam"} {
			t.Run(kind, func(t *testing.T) {
				fixture := s7AROwnedDivergenceFixture(t)
				s7ARReplaceArchiveBlobKind(t, fixture, kind)
				restore := func() {}
				if kind == "device-seam" {
					t.Log("device-node classification uses an injected execution seam because unprivileged CI cannot mknod")
					restore = s7ARInstallDeviceProbe(t, fixture.blobRel)
				}
				code, stdout, stderr, _ := runPrepare(
					t, "--path", fixture.root, "feature", "intent-archive", "purge", fixture.slug,
					"--blob", fixture.hash, "--yes", "--json",
				)
				restore()
				report := decodeIntentArchivePurgeReport(t, stdout)
				s7ARAssertBlobDivergence(t, fixture, kind, code, stdout, stderr, report)
				if report.Divergence == nil {
					t.Fatalf("PIB-506 %s missing divergence report", kind)
				}
			})
		}

		t.Run("index-strict-decode-stop", func(t *testing.T) {
			root, slug := s7ARArchiveSelectorFixture(t, false)
			_, index := readIntentArchiveCLIIndex(t, root, slug)
			hash := index.Generations[0].Replaced[0].ContentSHA256
			blobRel, _ := store.IntentArchiveBlobRel(slug, hash)
			indexRel, _ := store.IntentArchiveIndexRel(slug)
			fixture := s7ARDivergenceFixture{
				root: root, slug: slug, hash: hash, blobRel: blobRel, indexRel: indexRel,
			}
			indexAbs := filepath.Join(fixture.root, filepath.FromSlash(fixture.indexRel))
			previousAfterRename := s7APAfterPurgeIndexRename
			s7APAfterPurgeIndexRename = func(string) {
				if err := os.WriteFile(indexAbs, []byte("{broken"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			code, stdout, stderr, _ := runPrepare(
				t, "--path", fixture.root, "feature", "intent-archive", "purge", fixture.slug,
				"--blob", fixture.hash, "--yes", "--json",
			)
			s7APAfterPurgeIndexRename = previousAfterRename
			report := decodeIntentArchivePurgeReport(t, stdout)
			if code != 6 || report.Divergence == nil ||
				report.Divergence.Kind != "index" ||
				report.Divergence.PendingHash != fixture.hash ||
				report.Divergence.Index != fixture.indexRel ||
				report.Divergence.Blob != "" || report.Divergence.RemoveCommand != "" ||
				report.Divergence.Warning != "The archive index stopped strict-decoding after purge ownership was established." ||
				report.Divergence.RestoreInstruction != "Restore index.json to bytes that strict-decode using your own version control or backup, then rerun the purge. Do not remove index.json." ||
				report.Divergence.Cost != "Restoring the strict index preimage preserves the archive generations; removing index.json would discard them. Archived blob bytes that were ever committed remain in this repository's Git history; removing them from history is not something tpatch does." ||
				report.Divergence.Retry != "tpatch feature intent-archive purge "+fixture.slug+" --blob "+fixture.hash+" --yes --json" ||
				report.Divergence.RetryCWD != store.IntentArchiveRepairCWD ||
				len(report.Divergence.CompletedHashes) != 0 ||
				len(report.Divergence.RemainingHashes) != 0 ||
				!reflect.DeepEqual(report.Hashes, []string{fixture.hash}) ||
				len(report.References) != 1 ||
				report.References[0].Hash != fixture.hash ||
				len(report.Blobs) != 0 || len(report.OrphanBlobs) != 0 ||
				report.PurgeProgress != nil || report.PendingPurge != nil ||
				report.Recovery != nil || report.RemainingRepairs != nil ||
				strings.Contains(stdout, "--abandon-transaction") ||
				strings.Contains(stdout, fixture.root) {
				t.Fatalf("PIB-506 index divergence = exit:%d stderr:%q report:%+v",
					code, stderr, report)
			}
			s7ARAssertDivergenceEnvelope(t, fixture, report, stderr)
			human, _ := s7ARSplitHumanAndError(t, stderr)
			if human != s7ARExpectedDivergenceHuman(report) ||
				strings.Contains(human, "rm -rf") ||
				strings.Contains(human, "\n  blob:") ||
				strings.Count(human, prepareRetryHeader+"\n") != 1 {
				t.Fatalf("PIB-506 index human contract:\n%s", human)
			}
		})

		for _, tc := range []struct {
			name string
			live bool
		}{
			{name: "non-owned-tombstone-live-false", live: false},
			{name: "non-owned-tombstone-live-true", live: true},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root, slug := intentArchiveCLIWorkspace(t)
				data := []byte("AR non-owned tombstone\n")
				tombstone := intentArchiveCLIReplacement(
					t, store.IntentArchiveArtifactAnalysis, data, store.IntentArchiveWireTombstoned,
				)
				generations := []store.IntentArchiveGeneration{
					intentArchiveCLIGeneration(t, slug, tombstone),
				}
				if tc.live {
					retained := intentArchiveCLIReplacement(
						t, store.IntentArchiveArtifactSpec, data, store.IntentArchiveWireRetained,
					)
					generations = append(generations, intentArchiveCLIGeneration(t, slug, retained))
				}
				writeIntentArchiveCLIFixture(
					t, root, slug, intentArchiveCLIIndex(t, slug, generations...),
					map[string][]byte{tombstone.ContentSHA256: data},
				)
				args := []string{
					"--path", root, "feature", "intent-archive", "purge", slug,
					"--blob", tombstone.ContentSHA256, "--yes", "--json", "--quiet",
				}
				if tc.live {
					args = []string{
						"--path", root, "feature", "intent-archive", "purge", slug,
						"--orphans", "--yes", "--json", "--quiet",
					}
				}
				code, stdout, stderr, _ := runPrepare(t, args...)
				report := decodeIntentArchivePurgeReport(t, stdout)
				wantRetry := "tpatch feature intent-archive purge " + slug + " --orphans --yes"
				if tc.live {
					wantRetry = "tpatch feature intent-archive purge " + slug +
						" --blob " + tombstone.ContentSHA256 + " --yes"
				}
				want := s7ARExpectedNonOwnedTombstoneReport(
					t, slug, generations, tombstone, data, tc.live, wantRetry,
				)
				if code != 3 {
					t.Fatalf("PIB-506 non-owned tombstone live=%t exit=%d, want 3", tc.live, code)
				}
				if err := validateS7ARNonOwnedTombstoneReport(report, want); err != nil {
					t.Fatalf("PIB-506 non-owned tombstone live=%t: %v", tc.live, err)
				}
				s7ARAssertNonOwnedTombstoneJSONShape(t, stdout)
				wantStderr := "error: feature intent-archive purge " + slug +
					": refused archive-index-storage-inconsistent\n"
				if stderr != wantStderr {
					t.Fatalf("PIB-506 non-owned tombstone live=%t stderr=%q, want %q",
						tc.live, stderr, wantStderr)
				}
				for _, leaked := range []string{root, string(data), "--abandon-transaction", "--all --yes"} {
					if strings.Contains(stdout+stderr, leaked) {
						t.Fatalf("PIB-506 non-owned tombstone live=%t leaked %q", tc.live, leaked)
					}
				}
				for _, character := range stdout + stderr {
					if unicode.IsControl(character) && character != '\n' && character != '\t' {
						t.Fatalf("PIB-506 non-owned tombstone live=%t leaked control rune %#U",
							tc.live, character)
					}
				}
				for _, fixture := range []struct {
					name   string
					mutate func(*intentArchivePurgeReport)
				}{
					{
						name: "extra-stage",
						mutate: func(candidate *intentArchivePurgeReport) {
							candidate.RemainingRepairs.Stages = append(
								candidate.RemainingRepairs.Stages,
								candidate.RemainingRepairs.Stages[0],
							)
						},
					},
					{
						name: "wrong-route",
						mutate: func(candidate *intentArchivePurgeReport) {
							candidate.RemainingRepairs.Stages[0].Repair =
								"tpatch feature intent-archive purge " + slug + " --all --yes"
						},
					},
					{
						name: "wrong-envelope-metadata",
						mutate: func(candidate *intentArchivePurgeReport) {
							candidate.Action = "purge"
							candidate.Refusal.Retry = wantRetry
						},
					},
				} {
					candidate := s7ARExpectedNonOwnedTombstoneReport(
						t, slug, generations, tombstone, data, tc.live, wantRetry,
					)
					fixture.mutate(&candidate)
					if err := validateS7ARNonOwnedTombstoneReport(candidate, want); err == nil {
						t.Fatalf("PIB-506 same-shape validator accepted %s live=%t",
							fixture.name, tc.live)
					}
				}
			})
		}
	})

	t.Run("PIB-507", func(t *testing.T) {
		for _, kind := range []string{"regular", "symlink", "directory", "fifo", "device-seam"} {
			t.Run(kind, func(t *testing.T) {
				fixture := s7AROwnedDivergenceFixture(t)
				s7ARReplaceArchiveBlobKind(t, fixture, kind)
				restore := func() {}
				if kind == "device-seam" {
					t.Log("device-node removal uses the explicit injected classification limitation; the removal command still executes against its managed path")
					restore = s7ARInstallDeviceProbe(t, fixture.blobRel)
				}
				code, stdout, _, _ := runPrepare(
					t, "--path", fixture.root, "feature", "intent-archive", "purge", fixture.slug,
					"--blob", fixture.hash, "--yes", "--json", "--quiet",
				)
				restore()
				report := decodeIntentArchivePurgeReport(t, stdout)
				if code != 6 || report.Divergence == nil {
					t.Fatalf("PIB-507 initial %s divergence = exit:%d report:%+v", kind, code, report)
				}
				blobsDir := filepath.Join(
					fixture.root, ".tpatch", "features", fixture.slug,
					"artifacts", "intent-archive", "blobs",
				)
				beforeBlobs := s7ARSnapshotTree(t, blobsDir)
				beforeRoot := s7ARSnapshotTree(t, fixture.root)
				command := exec.Command("sh", "-c", report.Divergence.RemoveCommand)
				command.Dir = fixture.root
				if output, err := command.CombinedOutput(); err != nil {
					t.Fatalf("PIB-507 execute %s command: %v\n%s", kind, err, output)
				}
				afterBlobs := s7ARSnapshotTree(t, blobsDir)
				afterRoot := s7ARSnapshotTree(t, fixture.root)
				targetRel, err := filepath.Rel(blobsDir, filepath.Join(
					fixture.root, filepath.FromSlash(fixture.blobRel),
				))
				if err != nil {
					t.Fatal(err)
				}
				wantRemoved := map[string]bool{filepath.ToSlash(targetRel): true}
				if kind == "directory" {
					wantRemoved[filepath.ToSlash(filepath.Join(targetRel, "child.keep"))] = true
				}
				s7ARAssertExactRemovedPaths(t, "PIB-507 "+kind, beforeBlobs, afterBlobs, wantRemoved)
				s7ARAssertRootDeltaWithin(t, "PIB-507 "+kind, beforeRoot, afterRoot, fixture.blobRel)
				if _, err := os.Lstat(filepath.Join(
					fixture.root, filepath.FromSlash(fixture.blobRel),
				)); !os.IsNotExist(err) {
					t.Fatalf("PIB-507 %s managed path remains: %v", kind, err)
				}
				sibling := filepath.Join(filepath.Dir(fixture.targetPath), "sibling.keep")
				if data, err := os.ReadFile(sibling); err != nil || string(data) != "sibling\n" {
					t.Fatalf("PIB-507 %s sibling changed: %q err=%v", kind, data, err)
				}
				if kind == "symlink" {
					if data, err := os.ReadFile(fixture.targetPath); err != nil || string(data) != "target\n" {
						t.Fatalf("PIB-507 symlink target changed: %q err=%v", data, err)
					}
				}
				safeRel, _ := store.IntentArchiveBlobRel(fixture.slug, fixture.safeHash)
				safeBefore := s7APCaptureIdentity(t, fixture.root, safeRel)
				blobsBeforeRecovery := s7ARSnapshotTree(t, blobsDir)

				argv, err := s7APParseRenderedCommand(report.Divergence.Retry)
				if err != nil {
					t.Fatal(err)
				}
				code, stdout, stderr := s7APRunFromWorkspace(t, fixture.root, argv)
				recovered := decodeIntentArchivePurgeReport(t, stdout)
				if code != 0 || stderr != "" ||
					recovered.Outcome != string(store.IntentArchivePurgeRecovered) ||
					recovered.Recovery == nil ||
					recovered.Recovery.Kind != "archive-purge-finalize" ||
					!reflect.DeepEqual(recovered.Recovery.FinalizedHashes, []string{fixture.hash}) ||
					!s7APCaptureIdentity(t, fixture.root, safeRel).Equal(safeBefore) {
					t.Fatalf("PIB-507 %s recovery = exit:%d stderr:%q report:%+v",
						kind, code, stderr, recovered)
				}
				if !reflect.DeepEqual(blobsBeforeRecovery, s7ARSnapshotTree(t, blobsDir)) {
					t.Fatalf("PIB-507 %s absent-blob recovery attempted a removal or changed siblings", kind)
				}
				_, index := readIntentArchiveCLIIndex(t, fixture.root, fixture.slug)
				for _, generation := range index.Generations {
					for _, replacement := range generation.Replaced {
						if replacement.ContentSHA256 == fixture.hash &&
							(!replacement.Purged || replacement.PurgePending || replacement.Blob != "") {
							t.Fatalf("PIB-507 %s reference not tombstoned: %+v", kind, replacement)
						}
					}
				}

				code, stdout, stderr, _ = runPrepare(
					t, "--path", fixture.root, "feature", "intent-archive", "purge", fixture.slug,
					"--blob", fixture.safeHash, "--yes", "--json", "--quiet",
				)
				mutated := decodeIntentArchivePurgeReport(t, stdout)
				if code != 0 || stderr != "" ||
					mutated.Outcome != string(store.IntentArchivePurgePurged) ||
					!reflect.DeepEqual(mutated.Hashes, []string{fixture.safeHash}) {
					t.Fatalf("PIB-507 %s subsequent mutation = exit:%d stderr:%q report:%+v",
						kind, code, stderr, mutated)
				}
			})
		}
	})
}

func TestS7ARAbandonContracts(t *testing.T) {
	t.Run("PIB-509", func(t *testing.T) {
		for _, state := range []string{"absent-feature", "malformed-status", "unreadable-status"} {
			t.Run(state, func(t *testing.T) {
				root, slug := prepareS4Workspace(t, "AR abandon "+state)
				s6WriteJournalFixture(t, root, slug, "journal-corrupt")
				trace := s7ARInstallGitSpawnSpy(t, root)
				featureDir := filepath.Join(root, ".tpatch", "features", slug)
				switch state {
				case "absent-feature":
					if err := os.RemoveAll(featureDir); err != nil {
						t.Fatal(err)
					}
				case "malformed-status":
					if err := os.WriteFile(filepath.Join(featureDir, "status.json"), []byte("{bad"), 0o600); err != nil {
						t.Fatal(err)
					}
				case "unreadable-status":
					status := filepath.Join(featureDir, "status.json")
					if err := os.Remove(status); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(status, 0o700); err != nil {
						t.Fatal(err)
					}
				}
				before := s7AROptionalTree(t, featureDir)
				code, stdout, stderr, _ := runPrepare(
					t, "--path", root, "prepare", slug,
					"--abandon-transaction", "--yes", "--json", "--quiet",
				)
				report := prepareS4Report(t, stdout)
				after := s7AROptionalTree(t, featureDir)
				if code != 0 || stderr != "" || report.Outcome != "abandoned" ||
					report.Abandoned == nil || before != after ||
					s7ARContainsAny(stdout, "feature-not-found", "status-malformed", "status-unreadable") {
					t.Fatalf("PIB-509 %s = exit:%d stderr:%q report:%+v changed:%t",
						state, code, stderr, report, before != after)
				}
				s7ARAssertNoGitSpawn(t, trace)
				movedJournal := filepath.Join(
					root,
					filepath.FromSlash(strings.TrimSuffix(report.Abandoned.Directory, "/")),
					"journal.json",
				)
				if _, err := os.Stat(movedJournal); err != nil {
					t.Fatalf("PIB-509 %s evidence was not moved: %v", state, err)
				}
			})
		}
	})

	t.Run("PIB-510", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AR abandon ordering")
		s6WriteJournalFixture(t, root, slug, "journal-corrupt")
		if err := os.RemoveAll(filepath.Join(root, ".tpatch", "features", slug)); err != nil {
			t.Fatal(err)
		}
		sources := s7ARProductionSourceSet(t)
		if err := validateS7ARAbandonReadOrdering(sources); err != nil {
			t.Fatal(err)
		}
		trace := s7ARInstallGitSpawnSpy(t, root)
		oldBeforeLock := beforeLockAcquire
		oldBranch := beforeAbandonBranch
		oldProvider := prepareLoadProvider
		oldRedaction := beforeRedactionScan
		oldRecovery := afterRecoveryComplete
		var order []string
		providerLoads, redactionScans, recoveries := 0, 0, 0
		beforeLockAcquire = func() { order = append(order, "lock") }
		beforeAbandonBranch = func() { order = append(order, "branch") }
		prepareLoadProvider = func(repoStore *store.Store) (provider.Provider, provider.Config) {
			providerLoads++
			return oldProvider(repoStore)
		}
		beforeRedactionScan = func() { redactionScans++ }
		afterRecoveryComplete = func() { recoveries++ }
		defer func() {
			beforeLockAcquire = oldBeforeLock
			beforeAbandonBranch = oldBranch
			prepareLoadProvider = oldProvider
			beforeRedactionScan = oldRedaction
			afterRecoveryComplete = oldRecovery
		}()
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		if code != 0 || stderr != "" ||
			fmt.Sprint(order) != "[lock branch]" ||
			providerLoads != 0 || redactionScans != 0 || recoveries != 0 {
			t.Fatalf("PIB-510 ordering = exit:%d stderr:%q stdout:%q order:%v provider:%d redact:%d recovery:%d",
				code, stderr, stdout, order, providerLoads, redactionScans, recoveries)
		}
		s7ARAssertNoGitSpawn(t, trace)
	})

	t.Run("PIB-512", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AR child-held abandon")
		s6WriteJournalFixture(t, root, slug, "journal-corrupt")
		before := readTree(t, filepath.Join(root, ".tpatch", "local"))
		holder, closeHolder := s7StartCLIProcessHolder(t, root)
		defer closeHolder()
		for _, yes := range []bool{false, true} {
			args := []string{"--path", root, "prepare", slug, "--abandon-transaction", "--json", "--quiet"}
			if yes {
				args = append(args, "--yes")
			}
			code, stdout, stderr, _ := runPrepare(t, args...)
			report := prepareS4Report(t, stdout)
			text := ""
			if report.Refusal != nil {
				text = report.Refusal.Message + " " + report.Refusal.Remediation
			}
			wantMessage := "The workspace mutation authority is held by another mutating prepare or archive operation. The holder's identity is unknowable."
			wantRemediation := "Wait for the live operation to finish, then retry."
			wantStderr := "error: prepare " + slug + ": abandon refused transaction-in-progress\n"
			if code != 3 || stderr != wantStderr || report.Refusal == nil ||
				report.Refusal.Code != "transaction-in-progress" ||
				report.Refusal.Message != wantMessage ||
				report.Refusal.Remediation != wantRemediation ||
				report.Refusal.Retry != "" ||
				s7ARContainsAny(strings.ToLower(text), "rm ", "manual removal", ".tpatch/local/") ||
				!bytes.Equal(before, readTree(t, filepath.Join(root, ".tpatch", "local"))) {
				t.Fatalf("PIB-512 yes=%t = exit:%d stderr:%q report:%+v", yes, code, stderr, report)
			}
		}
		if holder.ProcessState != nil {
			t.Fatal("PIB-512 child holder exited before release")
		}
	})

	t.Run("PIB-513", func(t *testing.T) {
		root, slug := prepareS4Workspace(t, "AR combined abandon bypass")
		s6WriteJournalFixture(t, root, slug, "journal-corrupt")
		if err := os.RemoveAll(filepath.Join(root, ".tpatch", "features", slug)); err != nil {
			t.Fatal(err)
		}
		trace := s7ARInstallGitSpawnSpy(t, root)
		previousGitProcess := s7ARRunGitProcess
		gitAttempts := 0
		s7ARRunGitProcess = func(request s7ARGitProcessRequest) s7ARGitProcessResult {
			gitAttempts++
			return s7ARGitProcessResult{exitCode: -1, err: exec.ErrNotFound}
		}
		defer func() { s7ARRunGitProcess = previousGitProcess }()
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "prepare", slug,
			"--abandon-transaction", "--yes", "--json", "--quiet",
		)
		report := prepareS4Report(t, stdout)
		if code != 0 || stderr != "" || report.Outcome != "abandoned" ||
			report.Abandoned == nil || len(report.Abandoned.Moved) == 0 ||
			s7ARContainsAny(stdout, "journal-", "feature-not-found", "local-lane-") ||
			gitAttempts != 0 {
			t.Fatalf("PIB-513 = exit:%d stderr:%q report:%+v git-attempts:%d",
				code, stderr, report, gitAttempts)
		}
		s7ARAssertNoGitSpawn(t, trace)
		movedJournal := filepath.Join(
			root,
			filepath.FromSlash(strings.TrimSuffix(report.Abandoned.Directory, "/")),
			"journal.json",
		)
		if _, err := os.Stat(movedJournal); err != nil {
			t.Fatalf("PIB-513 corrupt evidence was not moved: %v", err)
		}
	})
}

func TestS7ARArchiveControlContracts(t *testing.T) {
	t.Run("PIB-514", func(t *testing.T) {
		root, slug := s7ARArchiveSelectorFixture(t, false)
		trace := s7ARInstallGitSpawnSpy(t, root)
		beforeList := s7ARSnapshotTree(t, root)
		code, _, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "list", slug, "--json", "--quiet",
		)
		if code != 0 || stderr != "" ||
			!reflect.DeepEqual(beforeList, s7ARSnapshotTree(t, root)) {
			t.Fatalf("PIB-514 list = exit:%d stderr:%q", code, stderr)
		}
		s7ARAssertNoGitSpawn(t, trace)

		for _, selector := range []string{"blob", "generation", "all", "orphans"} {
			for _, confirmed := range []bool{false, true} {
				root, slug = s7ARArchiveSelectorFixture(t, selector == "orphans")
				trace = s7ARInstallGitSpawnSpy(t, root)
				before := s7ARSnapshotTree(t, root)
				args := s7ARArchiveSelectorArgs(t, root, slug, selector)
				if confirmed {
					args = append(args, "--yes")
				}
				args = append(args, "--json", "--quiet")
				code, _, _, _ := runPrepare(t, args...)
				if code != 0 {
					t.Fatalf("PIB-514 %s confirmed=%t exit=%d", selector, confirmed, code)
				}
				s7ARAssertNoGitSpawn(t, trace)
				after := s7ARSnapshotTree(t, root)
				if !confirmed && !reflect.DeepEqual(before, after) {
					t.Fatalf("PIB-514 %s preview wrote workspace bytes: %v",
						selector, s7ARTreeDelta(before, after))
				}
				if confirmed {
					archivePrefix := ".tpatch/features/" + slug + "/artifacts/intent-archive"
					changedPaths := s7ARTreeDelta(before, after)
					if len(changedPaths) == 0 {
						t.Fatalf("PIB-514 %s confirmed purge changed no archive bytes", selector)
					}
					for _, changed := range changedPaths {
						if changed != archivePrefix &&
							!strings.HasPrefix(changed, archivePrefix+"/") {
							t.Fatalf("PIB-514 %s changed path outside selected archive: %s",
								selector, changed)
						}
						if strings.HasPrefix(changed, ".tpatch/local/") {
							t.Fatalf("PIB-514 %s changed local-lane path: %s", selector, changed)
						}
					}
				}
			}
		}
	})

	t.Run("PIB-515", func(t *testing.T) {
		for _, selector := range []string{"blob", "generation", "all", "orphans"} {
			fixture := s7AQPendingArchiveFixture(t)
			s7ARAssertWorkspaceSnapshotDetectsOutsideWrite(t, fixture.root)
			before := s7ARSnapshotTree(t, fixture.root)
			oldAcquire := intentArchiveAcquireAuthority
			acquires := 0
			intentArchiveAcquireAuthority = func(path string) (*intentlock.WorkspaceAuthority, error) {
				acquires++
				return oldAcquire(path)
			}
			args := s7ARPendingSelectorArgs(fixture, selector)
			code, stdout, stderr, _ := runPrepare(t, args...)
			intentArchiveAcquireAuthority = oldAcquire
			report := decodeIntentArchivePurgeReport(t, stdout)
			after := s7ARSnapshotTree(t, fixture.root)
			wantPending := make([]intentArchivePendingHashReport, 0, len(fixture.pendingHashes))
			indexRel, _ := store.IntentArchiveIndexRel(fixture.slug)
			for _, hash := range fixture.pendingHashes {
				blobRel, _ := store.IntentArchiveBlobRel(fixture.slug, hash)
				wantPending = append(wantPending, intentArchivePendingHashReport{
					Hash: hash, Blob: blobRel, Index: indexRel, Plan: intentArchivePendingPlan,
				})
			}
			if report.PendingPurge == nil {
				t.Fatalf("PIB-515 %s omitted pending_purge: exit:%d stderr:%q report:%+v",
					selector, code, stderr, report)
			}
			wantRetry := s7ARExpectedPendingRetryArgv(fixture, selector)
			gotRetry, retryErr := s7APParseRenderedCommand(report.PendingPurge.Retry)
			if code != 0 || stderr != "" ||
				report.Outcome != string(store.IntentArchivePurgeRecoveryRequired) ||
				report.PendingPurge == nil || !report.PendingPurge.RecoveryRequired ||
				report.PendingPurge.Selector != selector ||
				report.PendingPurge.RetryCWD != store.IntentArchiveRepairCWD ||
				retryErr != nil || !reflect.DeepEqual(gotRetry, wantRetry) ||
				!reflect.DeepEqual(report.PendingPurge.PendingHashes, wantPending) ||
				acquires != 0 || !reflect.DeepEqual(before, after) ||
				s7ARContainsAny(strings.ToLower(stdout), `"outcome":"recovered"`, "finalized") {
				t.Fatalf("PIB-515 %s = exit:%d stderr:%q report:%+v acquires:%d changed:%t",
					selector, code, stderr, report, acquires, !reflect.DeepEqual(before, after))
			}
			seen := map[string]bool{}
			for index, pending := range report.PendingPurge.PendingHashes {
				if index > 0 && report.PendingPurge.PendingHashes[index-1].Hash >= pending.Hash {
					t.Fatalf("PIB-515 %s pending hashes are not strictly sorted: %+v",
						selector, report.PendingPurge.PendingHashes)
				}
				if seen[pending.Hash] {
					t.Fatalf("PIB-515 %s duplicate pending hash %s", selector, pending.Hash)
				}
				seen[pending.Hash] = true
			}
			for _, forbidden := range []string{
				"--path", "--all", "--orphans", "--generation", "--blob",
			} {
				if forbidden == s7ARSelectorFlag(selector) {
					continue
				}
				if s7ARStringSliceContains(wantRetry, forbidden) {
					t.Fatalf("PIB-515 %s retry inherited/widened selector %s: %v",
						selector, forbidden, wantRetry)
				}
			}
		}
	})

	t.Run("PIB-516", func(t *testing.T) {
		root, slug := s7ARArchiveSelectorFixture(t, false)
		want := s7ARExpectedAllPartialReport(t, root, slug)
		s7ARFailPurgeBetweenHashes = func() error { return errors.New("AR fail between hashes") }
		defer func() { s7ARFailPurgeBetweenHashes = nil }()
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--all", "--yes", "--json",
		)
		s7ARFailPurgeBetweenHashes = nil
		report := decodeIntentArchivePurgeReport(t, stdout)
		if code != 5 {
			t.Fatalf("PIB-516 partial exit=%d, want 5", code)
		}
		if err := validateS7ARExactPartialReport(report, want); err != nil {
			t.Fatalf("PIB-516 partial: %v", err)
		}
		s7ARAssertExactPartialJSONShape(t, stdout, false, true)
		wantStderr := s7ARExpectedPartialHuman(want) +
			"error: feature intent-archive purge " + slug + ": purge-partial\n"
		if stderr != wantStderr {
			t.Fatalf("PIB-516 exact human/error mismatch\nwant:\n%s\ngot:\n%s", wantStderr, stderr)
		}
		remaining := append([]string(nil), report.PurgeProgress.RemainingHashes...)
		_, index := readIntentArchiveCLIIndex(t, root, slug)
		for _, generation := range index.Generations {
			for _, replacement := range generation.Replaced {
				if replacement.PurgePending {
					t.Fatalf("PIB-516 left pending reference: %+v", replacement)
				}
			}
		}
		argv, err := s7APParseRenderedCommand(report.PurgeProgress.Retry)
		if err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr = s7APRunFromWorkspace(t, root, argv)
		retry := decodeIntentArchivePurgeReport(t, stdout)
		if code != 0 ||
			retry.Outcome != string(store.IntentArchivePurgePurged) ||
			len(retry.Blobs) != len(remaining) ||
			retry.Blobs[0].Hash != remaining[0] ||
			strings.Contains(stdout, `"outcome":"recovered"`) ||
			!strings.HasPrefix(stderr, intentArchiveCommandPurge+" "+slug+": purged\n") ||
			strings.Contains(stderr, "error:") {
			t.Fatalf("PIB-516 retry = exit:%d stderr:%q report:%+v", code, stderr, retry)
		}
	})

	t.Run("PIB-517", func(t *testing.T) {
		root, slug := s7ARArchiveSelectorFixture(t, true)
		want := s7ARExpectedOrphanPartialReport(t, root, slug)
		indexRel, _ := store.IntentArchiveIndexRel(slug)
		indexAbs := filepath.Join(root, filepath.FromSlash(indexRel))
		beforeIndex, err := os.ReadFile(indexAbs)
		if err != nil {
			t.Fatal(err)
		}
		s7ARFailOrphanRemoveAfterFirst = func() error { return errors.New("AR fail after first orphan") }
		defer func() { s7ARFailOrphanRemoveAfterFirst = nil }()
		code, stdout, stderr, _ := runPrepare(
			t, "--path", root, "feature", "intent-archive", "purge", slug,
			"--orphans", "--yes", "--json",
		)
		s7ARFailOrphanRemoveAfterFirst = nil
		report := decodeIntentArchivePurgeReport(t, stdout)
		afterIndex, err := os.ReadFile(indexAbs)
		if err != nil {
			t.Fatal(err)
		}
		if code != 5 || !bytes.Equal(beforeIndex, afterIndex) {
			t.Fatalf("PIB-517 partial exit=%d indexChanged=%t", code, !bytes.Equal(beforeIndex, afterIndex))
		}
		if err := validateS7ARExactPartialReport(report, want); err != nil {
			t.Fatalf("PIB-517 partial: %v", err)
		}
		s7ARAssertExactPartialJSONShape(t, stdout, false, false)
		wantStderr := s7ARExpectedPartialHuman(want) +
			"error: feature intent-archive purge " + slug + ": purge-partial\n"
		if stderr != wantStderr {
			t.Fatalf("PIB-517 exact human/error mismatch\nwant:\n%s\ngot:\n%s", wantStderr, stderr)
		}
		remaining := append([]string(nil), report.PurgeProgress.RemainingHashes...)
		argv, err := s7APParseRenderedCommand(report.PurgeProgress.Retry)
		if err != nil {
			t.Fatal(err)
		}
		code, stdout, stderr = s7APRunFromWorkspace(t, root, argv)
		retry := decodeIntentArchivePurgeReport(t, stdout)
		if code != 0 ||
			retry.Outcome != string(store.IntentArchivePurgePurged) ||
			!reflect.DeepEqual(retry.Hashes, remaining) ||
			strings.Contains(stdout, `"outcome":"recovered"`) ||
			!strings.HasPrefix(stderr, intentArchiveCommandPurge+" "+slug+": purged\n") ||
			strings.Contains(stderr, "error:") {
			t.Fatalf("PIB-517 retry = exit:%d stderr:%q report:%+v", code, stderr, retry)
		}
	})
}

func s7AROwnedDivergenceFixture(t *testing.T) s7ARDivergenceFixture {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	ownedBytes := []byte("AR owned expected bytes\n")
	safeBytes := []byte("AR safe archive bytes\n")
	pending := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, ownedBytes, store.IntentArchiveWireRemovalPending,
	)
	retained := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, ownedBytes, store.IntentArchiveWireRetained,
	)
	tombstoned := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactExploration, ownedBytes, store.IntentArchiveWireTombstoned,
	)
	safe := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, safeBytes, store.IntentArchiveWireRetained,
	)
	index := intentArchiveCLIIndex(
		t, slug,
		intentArchiveCLIGeneration(t, slug, pending),
		intentArchiveCLIGeneration(t, slug, retained),
		intentArchiveCLIGeneration(t, slug, tombstoned),
		intentArchiveCLIGeneration(t, slug, safe),
	)
	writeIntentArchiveCLIFixture(t, root, slug, index, map[string][]byte{
		pending.ContentSHA256: ownedBytes,
		safe.ContentSHA256:    safeBytes,
	})
	blobRel, _ := store.IntentArchiveBlobRel(slug, pending.ContentSHA256)
	indexRel, _ := store.IntentArchiveIndexRel(slug)
	target := filepath.Join(root, "outside-target.keep")
	if err := os.WriteFile(target, []byte("target\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(filepath.Dir(target), "sibling.keep")
	if err := os.WriteFile(sibling, []byte("sibling\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return s7ARDivergenceFixture{
		root: root, slug: slug, hash: pending.ContentSHA256,
		blobRel: blobRel, indexRel: indexRel, safeHash: safe.ContentSHA256,
		targetPath: target,
	}
}

func s7ARReplaceArchiveBlobKind(t *testing.T, fixture s7ARDivergenceFixture, kind string) {
	t.Helper()
	path := filepath.Join(fixture.root, filepath.FromSlash(fixture.blobRel))
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	switch kind {
	case "regular":
		if err := os.WriteFile(path, []byte("wrong bytes\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "symlink":
		relative, err := filepath.Rel(filepath.Dir(path), fixture.targetPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(relative, path); err != nil {
			t.Fatal(err)
		}
	case "directory":
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(path, "child.keep"), []byte("child\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	case "fifo":
		if err := syscall.Mkfifo(path, 0o600); err != nil {
			t.Fatal(err)
		}
	case "device-seam":
		if err := os.WriteFile(path, []byte("device seam placeholder\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown divergence kind %q", kind)
	}
}

type s7ARDeviceProbeStorage struct {
	store.IntentArchiveStorage
	blobRel string
}

func (storage *s7ARDeviceProbeStorage) ProbeBlob(blobRel string) (store.IntentArchiveBlobProbe, error) {
	probe, err := storage.IntentArchiveStorage.ProbeBlob(blobRel)
	if err == nil && blobRel == storage.blobRel && probe.Kind != store.IntentArchiveBlobKindAbsent {
		probe.Kind = store.IntentArchiveBlobKindDevice
		probe.SHA256 = ""
	}
	return probe, err
}

func s7ARInstallDeviceProbe(t *testing.T, blobRel string) func() {
	t.Helper()
	previous := intentArchiveNewStorage
	intentArchiveNewStorage = func(
		authority *intentlock.WorkspaceAuthority,
		root *os.Root,
	) store.IntentArchiveStorage {
		return &s7ARDeviceProbeStorage{
			IntentArchiveStorage: previous(authority, root),
			blobRel:              blobRel,
		}
	}
	return func() { intentArchiveNewStorage = previous }
}

func s7ARAssertBlobDivergence(
	t *testing.T,
	fixture s7ARDivergenceFixture,
	kind string,
	code int,
	stdout, stderr string,
	report intentArchivePurgeReport,
) {
	t.Helper()
	wantWarning := "tpatch will not delete or overwrite bytes it cannot identify. WARNING: the next command permanently deletes whatever object is at the managed blob path, including directory contents, with no undo. If you want to keep that object, stop here and preserve it with tooling appropriate to its type; tpatch does not name a preservation command."
	wantCost := "What this costs: what you remove is gone, and this hash has no archived recovery material afterwards. If that blob was ever committed, it is still in this repository's Git history; removing it from history is not something tpatch does."
	wantRetry := "tpatch feature intent-archive purge " + fixture.slug +
		" --blob " + fixture.hash + " --yes --json"
	if code != 6 || report.Refusal == nil ||
		report.Refusal.Code != string(store.IntentArchiveCodePurgeEvidenceDivergent) ||
		report.Divergence == nil || report.Divergence.Kind != "blob" ||
		report.Divergence.PendingHash != fixture.hash ||
		report.Divergence.Blob != fixture.blobRel ||
		report.Divergence.Index != fixture.indexRel ||
		report.Divergence.Warning != wantWarning ||
		report.Divergence.RemoveCommand != "rm -rf -- "+fixture.blobRel ||
		report.Divergence.RestoreInstruction != "" ||
		report.Divergence.RetryCWD != store.IntentArchiveRepairCWD ||
		report.Divergence.Retry != wantRetry ||
		report.Divergence.Cost != wantCost ||
		len(report.Hashes) != 0 || len(report.GenerationIDs) != 0 ||
		len(report.References) != 0 || len(report.Blobs) != 0 ||
		len(report.OrphanBlobs) != 0 ||
		report.PurgeProgress != nil || report.PendingPurge != nil ||
		report.Recovery != nil || report.RemainingRepairs != nil ||
		strings.Contains(stdout, "--abandon-transaction") ||
		strings.Contains(stdout, fixture.root) ||
		strings.Contains(stdout, "*") {
		t.Fatalf("PIB-506 %s = exit:%d stderr:%q report:%+v", kind, code, stderr, report)
	}
	s7ARAssertDivergenceEnvelope(t, fixture, report, stderr)
	for _, forbidden := range []string{
		"cp ", "git show", "readlink ", "mv ", "rsync ", "tar ",
		"ln ", "install ", "dd ", "chmod ",
		"wrong bytes", "device seam placeholder", "target\n", "child.keep",
	} {
		if strings.Contains(strings.ToLower(stdout+stderr), forbidden) {
			t.Fatalf("PIB-506 %s leaked forbidden content/command %q", kind, forbidden)
		}
	}
	text, _ := s7ARSplitHumanAndError(t, stderr)
	if text != s7ARExpectedDivergenceHuman(report) {
		t.Fatalf("PIB-506 %s exact human mismatch:\n%s", kind, text)
	}
	if warningAt, commandAt := strings.Index(text, "WARNING:"), strings.Index(text, report.Divergence.RemoveCommand); warningAt < 0 || commandAt <= warningAt {
		t.Fatalf("PIB-506 %s warning/command order:\n%s", kind, text)
	}
	if strings.Count(text, report.Divergence.RemoveCommand+"\n") != 1 ||
		strings.Count(text, prepareRetryHeader+"\n") != 1 ||
		strings.Count(text, "  "+report.Divergence.Retry+"\n") != 1 ||
		!strings.Contains(text, report.Divergence.Cost+"\n") {
		t.Fatalf("PIB-506 %s incomplete exact human contract:\n%s", kind, text)
	}
}

func s7ARExpectedDivergenceHuman(report intentArchivePurgeReport) string {
	var output strings.Builder
	fmt.Fprintf(&output, "%s %s: refused %s\n",
		intentArchiveCommandPurge, report.Slug, report.Refusal.Code)
	fmt.Fprintf(&output, "selector: %s\n", report.Selector)
	fmt.Fprintf(&output, "confirmed: %t\n", report.Confirmed)
	if len(report.GenerationIDs) != 0 {
		output.WriteString("generations:\n")
		for _, generationID := range report.GenerationIDs {
			fmt.Fprintf(&output, "  %s\n", generationID)
		}
	}
	if len(report.Hashes) != 0 {
		output.WriteString("hashes:\n")
		for _, hash := range report.Hashes {
			fmt.Fprintf(&output, "  %s\n", hash)
		}
	}
	if len(report.References) != 0 {
		output.WriteString("references:\n")
		for _, reference := range report.References {
			fmt.Fprintf(&output, "  %s %s %s %s %s\n",
				reference.GenerationID, reference.ArtifactID, reference.Path,
				reference.Hash, reference.WireState)
		}
	}
	if len(report.Blobs) != 0 {
		output.WriteString("blobs:\n")
		for _, blob := range report.Blobs {
			fmt.Fprintf(&output, "  %s %s size=%d present=%t removed=%t\n",
				blob.Hash, blob.Path, blob.SizeBytes, blob.Present, blob.Removed)
		}
	}
	fmt.Fprintf(&output, "Archive divergence: %s\n", report.Divergence.Warning)
	fmt.Fprintf(&output, "  pending hash: %s\n", report.Divergence.PendingHash)
	if report.Divergence.Blob != "" {
		fmt.Fprintf(&output, "  blob:  %s\n", report.Divergence.Blob)
	}
	fmt.Fprintf(&output, "  index: %s\n", report.Divergence.Index)
	if report.Divergence.RemoveCommand != "" {
		fmt.Fprintf(&output, "  %s\n", report.Divergence.RemoveCommand)
	}
	if report.Divergence.RestoreInstruction != "" {
		fmt.Fprintf(&output, "  %s\n", report.Divergence.RestoreInstruction)
	}
	for _, hash := range report.Divergence.CompletedHashes {
		fmt.Fprintf(&output, "  completed %s\n", hash)
	}
	for _, hash := range report.Divergence.RemainingHashes {
		fmt.Fprintf(&output, "  remaining %s\n", hash)
	}
	fmt.Fprintln(&output, report.Divergence.Cost)
	fmt.Fprintln(&output, prepareRetryHeader)
	fmt.Fprintf(&output, "  %s\n", report.Divergence.Retry)
	fmt.Fprintf(&output, "Refusal: %s\n", report.Refusal.Code)
	fmt.Fprintf(&output, "  %s\n", report.Refusal.Message)
	fmt.Fprintf(&output, "  %s\n", report.Refusal.Remediation)
	fmt.Fprintln(&output, report.HistoryDisclosure)
	return output.String()
}

func s7ARAssertDivergenceEnvelope(
	t *testing.T,
	fixture s7ARDivergenceFixture,
	report intentArchivePurgeReport,
	stderr string,
) {
	t.Helper()
	if report.SchemaVersion != 1 ||
		report.Command != intentArchiveCommandPurge ||
		report.Slug != fixture.slug ||
		report.Outcome != "refused" ||
		report.Action != "none" ||
		report.Selector != "blob" ||
		!report.Confirmed ||
		report.Refusal == nil ||
		report.Refusal.Code != string(store.IntentArchiveCodePurgeEvidenceDivergent) ||
		report.Refusal.Message != "The intent archive refused the requested operation. Hash: "+fixture.hash+"." ||
		report.Refusal.Remediation != "Follow the archive-specific divergence procedure in this report; prepare recovery modes cannot repair an archive purge." ||
		report.Refusal.Retry != "" ||
		report.Refusal.RetryCWD != "" ||
		len(report.Advisories) != 0 ||
		report.BlastRadius != "" ||
		report.Retry != "" ||
		report.RetryCWD != "" ||
		report.HistoryDisclosure != intentArchiveHistoryDisclosure {
		t.Fatalf("PIB-506 incomplete divergence envelope: %+v", report)
	}
	_, errorLine := s7ARSplitHumanAndError(t, stderr)
	wantError := "error: feature intent-archive purge " + fixture.slug +
		": refused archive-purge-evidence-divergent\n"
	if errorLine != wantError {
		t.Fatalf("PIB-506 stderr error envelope = %q, want %q", errorLine, wantError)
	}
}

func s7ARExpectedNonOwnedTombstoneReport(
	t *testing.T,
	slug string,
	generations []store.IntentArchiveGeneration,
	tombstone store.IntentArchiveReplacement,
	data []byte,
	live bool,
	repair string,
) intentArchivePurgeReport {
	t.Helper()
	blobRel, err := store.IntentArchiveBlobRel(slug, tombstone.ContentSHA256)
	if err != nil {
		t.Fatal(err)
	}
	class := store.IntentArchiveRepairUnreferencedResidue
	selector := "blob"
	hashes := []string{tombstone.ContentSHA256}
	references := []intentArchivePurgeReferenceReport{{
		GenerationID: generations[0].GenerationID,
		ArtifactID:   string(tombstone.ArtifactID),
		Path:         ".tpatch/features/" + slug + "/" + tombstone.Path,
		Hash:         tombstone.ContentSHA256,
		WireState:    string(store.IntentArchiveWireTombstoned),
	}}
	blobs := []intentArchivePurgeBlobReport{{
		Hash:      tombstone.ContentSHA256,
		Path:      blobRel,
		SizeBytes: int64(len(data)),
		Present:   true,
		Removed:   false,
	}}
	orphans := []string{tombstone.ContentSHA256}
	if live {
		class = store.IntentArchiveRepairMixedReference
		selector = "orphans"
		hashes = []string{}
		references = []intentArchivePurgeReferenceReport{}
		blobs = []intentArchivePurgeBlobReport{}
		orphans = []string{}
	}
	stage := intentArchiveRepairStageReport{
		Ordinal:           1,
		Kind:              string(store.IntentArchiveRepairStagePurge),
		Class:             string(class),
		Hashes:            []string{tombstone.ContentSHA256},
		Paths:             []string{blobRel},
		Repair:            repair,
		RepairCWD:         store.IntentArchiveRepairCWD,
		ResultingClasses:  []string{},
		AfterPrerequisite: false,
	}
	return intentArchivePurgeReport{
		SchemaVersion:     1,
		Command:           intentArchiveCommandPurge,
		Slug:              slug,
		Outcome:           "refused",
		Action:            "none",
		Selector:          selector,
		Confirmed:         true,
		Hashes:            hashes,
		GenerationIDs:     []string{},
		References:        references,
		Blobs:             blobs,
		OrphanBlobs:       orphans,
		Advisories:        []prepareAdvisoryReport{},
		HistoryDisclosure: intentArchiveHistoryDisclosure,
		Refusal: &intentArchiveRefusalReport{
			Code: string(store.IntentArchiveCodeIndexStorageInconsistent),
			Message: "The intent archive refused the requested operation. Hash: " +
				tombstone.ContentSHA256 + ".",
			Remediation: "Follow the 1 structured remaining-repair stage(s) in rank order. " +
				"Complete every manual prerequisite before running any tpatch retry printed below it.",
		},
		RemainingRepairs: &intentArchiveRemainingRepairsReport{
			RerunRequired:   true,
			StagesRemaining: 1,
			NextStage: &intentArchiveRepairNextReport{
				Ordinal: 1,
				Kind:    string(store.IntentArchiveRepairStagePurge),
				Class:   string(class),
			},
			Stages: []intentArchiveRepairStageReport{stage},
		},
	}
}

func validateS7ARNonOwnedTombstoneReport(
	got, want intentArchivePurgeReport,
) error {
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("report mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	return nil
}

func s7ARAssertNonOwnedTombstoneJSONShape(t *testing.T, raw string) {
	t.Helper()
	top := s7ARDecodeJSONObject(t, raw)
	s7ARAssertJSONObjectKeys(t, "PIB-506 report", top, []string{
		"action", "advisories", "blobs", "command", "confirmed",
		"generation_ids", "hashes", "history_disclosure", "orphan_blobs",
		"outcome", "references", "refusal", "remaining_repairs", "schema_version",
		"selector", "slug",
	})
	refusal := s7ARDecodeJSONChild(t, top, "refusal")
	s7ARAssertJSONObjectKeys(t, "PIB-506 refusal", refusal, []string{
		"code", "message", "remediation",
	})
	remaining := s7ARDecodeJSONChild(t, top, "remaining_repairs")
	s7ARAssertJSONObjectKeys(t, "PIB-506 remaining_repairs", remaining, []string{
		"next_stage", "rerun_required", "stages", "stages_remaining",
	})
	next := s7ARDecodeJSONChild(t, remaining, "next_stage")
	s7ARAssertJSONObjectKeys(t, "PIB-506 next_stage", next, []string{
		"class", "kind", "ordinal",
	})
	var stages []map[string]json.RawMessage
	if err := json.Unmarshal(remaining["stages"], &stages); err != nil {
		t.Fatal(err)
	}
	if len(stages) != 1 {
		t.Fatalf("PIB-506 stages JSON count = %d, want 1", len(stages))
	}
	s7ARAssertJSONObjectKeys(t, "PIB-506 stage", stages[0], []string{
		"after_prerequisite", "class", "hashes", "kind", "ordinal",
		"paths", "repair", "repair_cwd", "resulting_classes",
	})
}

func s7ARExpectedAllPartialReport(t *testing.T, root, slug string) intentArchivePurgeReport {
	t.Helper()
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	replacements := map[string]store.IntentArchiveReplacement{}
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			replacements[replacement.ContentSHA256] = replacement
		}
	}
	hashes := make([]string, 0, len(replacements))
	for hash := range replacements {
		hashes = append(hashes, hash)
	}
	sort.Strings(hashes)
	if len(hashes) != 2 || len(index.Generations) != 1 {
		t.Fatalf("PIB-516 fixture hashes/generations = %d/%d, want 2/1",
			len(hashes), len(index.Generations))
	}
	references := make([]intentArchivePurgeReferenceReport, 0, len(hashes))
	blobs := make([]intentArchivePurgeBlobReport, 0, len(hashes))
	for position, hash := range hashes {
		replacement := replacements[hash]
		blobRel, err := store.IntentArchiveBlobRel(slug, hash)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(blobRel)))
		if err != nil {
			t.Fatal(err)
		}
		references = append(references, intentArchivePurgeReferenceReport{
			GenerationID: index.Generations[0].GenerationID,
			ArtifactID:   string(replacement.ArtifactID),
			Path:         ".tpatch/features/" + slug + "/" + replacement.Path,
			Hash:         hash,
			WireState:    string(store.IntentArchiveWireRetained),
		})
		blobs = append(blobs, intentArchivePurgeBlobReport{
			Hash:      hash,
			Path:      blobRel,
			SizeBytes: info.Size(),
			Present:   position != 0,
			Removed:   position == 0,
		})
	}
	return s7ARExpectedPartialReport(
		slug,
		"all",
		hashes,
		[]string{index.Generations[0].GenerationID},
		references,
		blobs,
		[]string{},
		store.IntentArchiveResumeCompletionOnly,
		hashes[0],
		hashes[1],
	)
}

func s7ARExpectedOrphanPartialReport(t *testing.T, root, slug string) intentArchivePurgeReport {
	t.Helper()
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	referenced := map[string]bool{}
	for _, generation := range index.Generations {
		for _, replacement := range generation.Replaced {
			referenced[replacement.ContentSHA256] = true
		}
	}
	blobsRel, err := store.IntentArchiveBlobsRel(slug)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(blobsRel)))
	if err != nil {
		t.Fatal(err)
	}
	var hashes []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".blob") {
			continue
		}
		hash := strings.TrimSuffix(name, ".blob")
		if !referenced[hash] {
			hashes = append(hashes, hash)
		}
	}
	sort.Strings(hashes)
	if len(hashes) != 2 {
		t.Fatalf("PIB-517 fixture orphan count = %d, want 2", len(hashes))
	}
	blobs := make([]intentArchivePurgeBlobReport, 0, len(hashes))
	for position, hash := range hashes {
		blobRel, err := store.IntentArchiveBlobRel(slug, hash)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(filepath.Join(root, filepath.FromSlash(blobRel)))
		if err != nil {
			t.Fatal(err)
		}
		blobs = append(blobs, intentArchivePurgeBlobReport{
			Hash:      hash,
			Path:      blobRel,
			SizeBytes: info.Size(),
			Present:   position != 0,
			Removed:   position == 0,
		})
	}
	return s7ARExpectedPartialReport(
		slug,
		"orphans",
		hashes,
		[]string{},
		[]intentArchivePurgeReferenceReport{},
		blobs,
		[]string{hashes[1]},
		store.IntentArchiveResumeOrphanScan,
		hashes[0],
		hashes[1],
	)
}

func s7ARExpectedPartialReport(
	slug, selector string,
	hashes, generationIDs []string,
	references []intentArchivePurgeReferenceReport,
	blobs []intentArchivePurgeBlobReport,
	orphans []string,
	resume store.IntentArchivePurgeResume,
	completed, remaining string,
) intentArchivePurgeReport {
	report := intentArchivePurgeReport{
		SchemaVersion:     1,
		Command:           intentArchiveCommandPurge,
		Slug:              slug,
		Outcome:           string(store.IntentArchivePurgePartial),
		Action:            "none",
		Selector:          selector,
		Confirmed:         true,
		Hashes:            append([]string{}, hashes...),
		GenerationIDs:     append([]string{}, generationIDs...),
		References:        append([]intentArchivePurgeReferenceReport{}, references...),
		Blobs:             append([]intentArchivePurgeBlobReport{}, blobs...),
		OrphanBlobs:       append([]string{}, orphans...),
		Advisories:        []prepareAdvisoryReport{},
		HistoryDisclosure: intentArchiveHistoryDisclosure,
		PurgeProgress: &intentArchivePurgeProgressReport{
			CompletedHashes: []string{completed},
			RemainingHashes: []string{remaining},
			Resume:          string(resume),
			Retry: "tpatch feature intent-archive purge " + slug +
				" --" + selector + " --yes --json",
			RetryCWD: store.IntentArchiveRepairCWD,
			State:    store.IntentArchivePurgeStateConsistent,
		},
	}
	if selector == "all" {
		report.BlastRadius = "The --all selector tombstones every reference in every generation and removes every blob in this archive. The unconfirmed preview is the default; repeated --blob selectors are the narrower alternative."
	}
	return report
}

func validateS7ARExactPartialReport(got, want intentArchivePurgeReport) error {
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("partial report mismatch\ngot:  %#v\nwant: %#v", got, want)
	}
	return nil
}

func s7ARAssertExactPartialJSONShape(
	t *testing.T,
	raw string,
	wantPending, wantBlastRadius bool,
) {
	t.Helper()
	top := s7ARDecodeJSONObject(t, raw)
	wantTop := []string{
		"action", "advisories", "blobs", "command", "confirmed",
		"generation_ids", "hashes", "history_disclosure", "orphan_blobs",
		"outcome", "purge_progress", "references", "schema_version", "selector", "slug",
	}
	if wantBlastRadius {
		wantTop = append(wantTop, "blast_radius")
	}
	s7ARAssertJSONObjectKeys(t, "partial purge report", top, wantTop)
	progress := s7ARDecodeJSONChild(t, top, "purge_progress")
	wantProgress := []string{
		"completed_hashes", "remaining_hashes", "resume", "retry", "retry_cwd", "state",
	}
	if wantPending {
		wantProgress = append(wantProgress, "pending_hash")
	}
	s7ARAssertJSONObjectKeys(t, "partial purge progress", progress, wantProgress)
}

func s7ARDecodeJSONObject(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &object); err != nil {
		t.Fatal(err)
	}
	return object
}

func s7ARDecodeJSONChild(
	t *testing.T,
	parent map[string]json.RawMessage,
	key string,
) map[string]json.RawMessage {
	t.Helper()
	raw, present := parent[key]
	if !present {
		t.Fatalf("JSON object omitted %s", key)
	}
	var child map[string]json.RawMessage
	if err := json.Unmarshal(raw, &child); err != nil {
		t.Fatalf("decode JSON object %s: %v", key, err)
	}
	return child
}

func s7ARAssertJSONObjectKeys(
	t *testing.T,
	label string,
	object map[string]json.RawMessage,
	want []string,
) {
	t.Helper()
	got := make([]string, 0, len(object))
	for key := range object {
		got = append(got, key)
	}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s keys = %v, want %v", label, got, want)
	}
}

func s7ARExpectedPartialHuman(report intentArchivePurgeReport) string {
	return s7ARRenderPartialHuman(
		report,
		intentArchivePurgeResumeInstruction(report.PurgeProgress.Resume),
	)
}

func s7ARRenderPartialHuman(
	report intentArchivePurgeReport,
	instruction string,
) string {
	var output strings.Builder
	fmt.Fprintf(&output, "%s %s: %s\n", report.Command, report.Slug, report.Outcome)
	fmt.Fprintf(&output, "selector: %s\n", report.Selector)
	fmt.Fprintf(&output, "confirmed: %t\n", report.Confirmed)
	if len(report.GenerationIDs) != 0 {
		output.WriteString("generations:\n")
		for _, generationID := range report.GenerationIDs {
			fmt.Fprintf(&output, "  %s\n", generationID)
		}
	}
	if len(report.Hashes) != 0 {
		output.WriteString("hashes:\n")
		for _, hash := range report.Hashes {
			fmt.Fprintf(&output, "  %s\n", hash)
		}
	}
	if len(report.References) != 0 {
		output.WriteString("references:\n")
		for _, reference := range report.References {
			fmt.Fprintf(&output, "  %s %s %s %s %s\n",
				reference.GenerationID, reference.ArtifactID, reference.Path,
				reference.Hash, reference.WireState)
		}
	}
	if len(report.Blobs) != 0 {
		output.WriteString("blobs:\n")
		for _, blob := range report.Blobs {
			fmt.Fprintf(&output, "  %s %s size=%d present=%t removed=%t\n",
				blob.Hash, blob.Path, blob.SizeBytes, blob.Present, blob.Removed)
		}
	}
	if len(report.OrphanBlobs) != 0 {
		output.WriteString("orphan blobs:\n")
		for _, hash := range report.OrphanBlobs {
			fmt.Fprintf(&output, "  %s\n", hash)
		}
	}
	if report.BlastRadius != "" {
		fmt.Fprintln(&output, report.BlastRadius)
	}
	fmt.Fprintf(&output, "Purge state: %s\n", report.PurgeProgress.State)
	fmt.Fprintf(&output, "Resume: %s\n", report.PurgeProgress.Resume)
	for _, hash := range report.PurgeProgress.CompletedHashes {
		fmt.Fprintf(&output, "  completed %s\n", hash)
	}
	if report.PurgeProgress.PendingHash != "" {
		fmt.Fprintf(&output, "  pending %s\n", report.PurgeProgress.PendingHash)
	}
	for _, hash := range report.PurgeProgress.RemainingHashes {
		fmt.Fprintf(&output, "  remaining %s\n", hash)
	}
	fmt.Fprintln(&output, instruction)
	fmt.Fprintln(&output, prepareRetryHeader)
	fmt.Fprintf(&output, "  %s\n", report.PurgeProgress.Retry)
	fmt.Fprintln(&output, report.HistoryDisclosure)
	return output.String()
}

func s7ARSplitHumanAndError(t *testing.T, stderr string) (string, string) {
	t.Helper()
	at := strings.LastIndex(stderr, "\nerror: ")
	if at < 0 {
		if strings.HasPrefix(stderr, "error: ") {
			return "", stderr
		}
		t.Fatalf("stderr lacks terminal error envelope: %q", stderr)
	}
	return stderr[:at+1], stderr[at+1:]
}

func s7AROptionalTree(t *testing.T, path string) string {
	t.Helper()
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return "<absent>"
	} else if err != nil {
		t.Fatal(err)
	}
	return string(readTree(t, path))
}

func s7ARArchiveSelectorFixture(t *testing.T, withOrphans bool) (string, string) {
	t.Helper()
	root, slug := intentArchiveCLIWorkspace(t)
	firstBytes := []byte("AR selector first\n")
	secondBytes := []byte("AR selector second\n")
	first := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactAnalysis, firstBytes, store.IntentArchiveWireRetained,
	)
	second := intentArchiveCLIReplacement(
		t, store.IntentArchiveArtifactSpec, secondBytes, store.IntentArchiveWireRetained,
	)
	blobs := map[string][]byte{
		first.ContentSHA256:  firstBytes,
		second.ContentSHA256: secondBytes,
	}
	if withOrphans {
		for _, data := range [][]byte{[]byte("AR orphan first\n"), []byte("AR orphan second\n")} {
			sum := sha256.Sum256(data)
			blobs[hex.EncodeToString(sum[:])] = data
		}
	}
	writeIntentArchiveCLIFixture(
		t, root, slug,
		intentArchiveCLIIndex(t, slug, intentArchiveCLIGeneration(t, slug, first, second)),
		blobs,
	)
	return root, slug
}

func s7ARArchiveSelectorArgs(t *testing.T, root, slug, selector string) []string {
	t.Helper()
	args := []string{"--path", root, "feature", "intent-archive", "purge", slug}
	_, index := readIntentArchiveCLIIndex(t, root, slug)
	switch selector {
	case "blob":
		args = append(args, "--blob", index.Generations[0].Replaced[0].ContentSHA256)
	case "generation":
		args = append(args, "--generation", index.Generations[0].GenerationID)
	case "all":
		args = append(args, "--all")
	case "orphans":
		args = append(args, "--orphans")
	default:
		t.Fatalf("unknown selector %q", selector)
	}
	return args
}

func s7ARPendingSelectorArgs(fixture s7AQPendingArchiveState, selector string) []string {
	args := []string{
		"--path", fixture.root, "feature", "intent-archive", "purge", fixture.slug,
	}
	switch selector {
	case "blob":
		args = append(args, "--blob", fixture.selectorHash)
	case "generation":
		args = append(args, "--generation", fixture.index.Generations[0].GenerationID)
	case "all":
		args = append(args, "--all")
	case "orphans":
		args = append(args, "--orphans")
	}
	return append(args, "--json", "--quiet")
}

func s7ARPendingSelectorFragment(fixture s7AQPendingArchiveState, selector string) string {
	switch selector {
	case "blob":
		return "--blob " + fixture.selectorHash
	case "generation":
		return "--generation " + fixture.index.Generations[0].GenerationID
	case "all":
		return "--all"
	case "orphans":
		return "--orphans"
	default:
		return "<unknown-selector>"
	}
}

func s7ARExpectedPendingRetryArgv(
	fixture s7AQPendingArchiveState,
	selector string,
) []string {
	argv := []string{"feature", "intent-archive", "purge", fixture.slug}
	switch selector {
	case "blob":
		argv = append(argv, "--blob", fixture.selectorHash)
	case "generation":
		argv = append(argv, "--generation", fixture.index.Generations[0].GenerationID)
	case "all":
		argv = append(argv, "--all")
	case "orphans":
		argv = append(argv, "--orphans")
	default:
		return nil
	}
	return append(argv, "--yes", "--json", "--quiet")
}

func s7ARAssertWorkspaceSnapshotDetectsOutsideWrite(t *testing.T, root string) {
	t.Helper()
	before := s7ARSnapshotTree(t, root)
	probe := filepath.Join(root, "s7-ar-preview-write-probe")
	if err := os.WriteFile(probe, []byte("outside .tpatch\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	after := s7ARSnapshotTree(t, root)
	if reflect.DeepEqual(before, after) ||
		!reflect.DeepEqual(s7ARTreeDelta(before, after), []string{"s7-ar-preview-write-probe"}) {
		t.Fatalf("PIB-515 whole-workspace snapshot missed outside write: %v",
			s7ARTreeDelta(before, after))
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, s7ARSnapshotTree(t, root)) {
		t.Fatal("PIB-515 outside-write sensitivity did not restore the fixture")
	}
}

func s7ARSelectorFlag(selector string) string {
	if selector == "all" || selector == "orphans" {
		return "--" + selector
	}
	return "--" + selector
}

func s7ARStringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

type s7ARTreeState map[string]string

func s7ARSnapshotTree(t *testing.T, root string) s7ARTreeState {
	t.Helper()
	state := s7ARTreeState{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			state[rel] = fmt.Sprintf("dir:%04o", info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				return err
			}
			state[rel] = fmt.Sprintf("symlink:%04o:%s", info.Mode().Perm(), target)
		case info.Mode().IsRegular():
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			sum := sha256.Sum256(body)
			state[rel] = fmt.Sprintf("file:%04o:%d:%x", info.Mode().Perm(), len(body), sum)
		default:
			state[rel] = "other:" + info.Mode().String()
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return state
}

func s7ARTreeDelta(before, after s7ARTreeState) []string {
	keys := map[string]bool{}
	for path := range before {
		keys[path] = true
	}
	for path := range after {
		keys[path] = true
	}
	var changed []string
	for path := range keys {
		if before[path] != after[path] {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func s7ARAssertExactRemovedPaths(
	t *testing.T,
	label string,
	before, after s7ARTreeState,
	wantRemoved map[string]bool,
) {
	t.Helper()
	for _, changed := range s7ARTreeDelta(before, after) {
		if !wantRemoved[changed] || after[changed] != "" {
			t.Fatalf("%s changed unexpected path %q: before=%q after=%q",
				label, changed, before[changed], after[changed])
		}
		delete(wantRemoved, changed)
	}
	if len(wantRemoved) != 0 {
		t.Fatalf("%s did not remove exact paths: %v", label, wantRemoved)
	}
}

func s7ARAssertRootDeltaWithin(
	t *testing.T,
	label string,
	before, after s7ARTreeState,
	allowed string,
) {
	t.Helper()
	allowed = filepath.ToSlash(allowed)
	for _, changed := range s7ARTreeDelta(before, after) {
		if changed != allowed && !strings.HasPrefix(changed, allowed+"/") {
			t.Fatalf("%s changed outside %q: %s", label, allowed, changed)
		}
	}
}

func s7ARInstallGitSpawnSpy(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(root, "git-spy")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	trace := filepath.Join(root, "git-spawn.trace")
	script := "#!/bin/sh\nprintf 'spawned\\n' >> \"$TPATCH_S7_AR_GIT_TRACE\"\nexit 93\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin)
	t.Setenv("TPATCH_S7_AR_GIT_TRACE", trace)
	return trace
}

func s7ARAssertNoGitSpawn(t *testing.T, trace string) {
	t.Helper()
	if _, err := os.Stat(trace); !os.IsNotExist(err) {
		t.Fatalf("unexpected Git process trace: %v", err)
	}
}

func s7ARArchiveOutsideSnapshot(t *testing.T, root, slug string) string {
	t.Helper()
	tpatch := filepath.Join(root, ".tpatch")
	archiveRel := filepath.FromSlash(
		"features/" + slug + "/artifacts/intent-archive",
	)
	var output bytes.Buffer
	err := filepath.WalkDir(tpatch, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(tpatch, path)
		if err != nil {
			return err
		}
		if rel == archiveRel {
			output.WriteString("D " + filepath.ToSlash(rel) + "\n")
			return filepath.SkipDir
		}
		if entry.IsDir() {
			output.WriteString("D " + filepath.ToSlash(rel) + "\n")
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		output.WriteString("F " + filepath.ToSlash(rel) + "\x00")
		output.Write(data)
		output.WriteByte('\n')
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func s7ARContainsAny(text string, values ...string) bool {
	for _, value := range values {
		if strings.Contains(text, value) {
			return true
		}
	}
	return false
}
