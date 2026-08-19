package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tesseracode/tesserapatch/internal/intent"
	"github.com/tesseracode/tesserapatch/internal/safety"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const (
	doctorD9PrepareLaneRel = ".tpatch/local/intent-prepare"
	doctorD9FeaturesRel    = ".tpatch/features"
)

var doctorD9PersistentEvidenceCodes = [...]string{
	"archive-blob-corrupt",
	"archive-blob-dangling",
	"archive-index-invalid",
	"archive-index-storage-inconsistent",
	"archive-orphan",
	"archive-purge-pending",
	"persistent-evidence-unsafe",
	"prepare-abandoned-evidence",
	"prepare-preimage-stale",
	"prepare-stage-stale",
	"prepare-temp-stale",
	"prepare-transaction-pending",
}

var newDoctorD9Boundary = func(root string) doctorD9Boundary {
	return &doctorD9OSBoundary{root: root}
}

// doctorD9Boundary is the complete D9 capability surface. The forbidden
// methods make zero-authority/process/write tests runtime-connected while the
// production implementation keeps those capabilities unavailable.
type doctorD9Boundary interface {
	Lstat(rel string) (fs.FileInfo, error)
	ReadDir(rel string) ([]os.DirEntry, error)
	ReadRegular(rel string, maxBytes int64) ([]byte, fs.FileInfo, error)
	OpenRoot(string) error
	OpenDot() error
	Control() error
	Flock() error
	Fstatfs() error
	Unlock() error
	RunProcess(string, ...string) error
	Write(string, []byte) error
}

type doctorD9OSBoundary struct {
	root       string
	beforeOpen func(string)
	afterOpen  func(string)
	afterRead  func(string)
}

func (boundary *doctorD9OSBoundary) Lstat(rel string) (fs.FileInfo, error) {
	before, err := boundary.inspectPath(rel)
	if err != nil {
		return nil, err
	}
	if doctorD9RefusedInfo(before) || (!before.Mode().IsRegular() && !before.IsDir()) {
		after, verifyErr := boundary.inspectPath(rel)
		if verifyErr != nil || !doctorD9SameObject(before, after) {
			return nil, errors.New("doctor D9 path identity changed")
		}
		return after, nil
	}
	file, opened, err := boundary.openConfined(rel)
	if err != nil {
		return nil, err
	}
	if closeErr := file.Close(); closeErr != nil {
		return nil, closeErr
	}
	return opened, nil
}

func (boundary *doctorD9OSBoundary) ReadDir(rel string) ([]os.DirEntry, error) {
	directory, opened, err := boundary.openConfined(rel)
	if err != nil {
		return nil, err
	}
	defer directory.Close()
	if !opened.IsDir() {
		return nil, errors.New("doctor D9 directory boundary refused")
	}
	entries, err := directory.ReadDir(-1)
	if err != nil {
		return nil, err
	}
	if boundary.afterRead != nil {
		boundary.afterRead(rel)
	}
	if err := boundary.verifyOpenedPath(rel, opened, false); err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, nil
}

func (boundary *doctorD9OSBoundary) ReadRegular(rel string, maxBytes int64) ([]byte, fs.FileInfo, error) {
	file, opened, err := boundary.openConfined(rel)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()
	if !opened.Mode().IsRegular() {
		return nil, opened, errors.New("doctor D9 regular-file boundary refused")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, opened, err
	}
	if int64(len(data)) > maxBytes {
		return nil, opened, errors.New("doctor D9 bounded read exceeded")
	}
	if boundary.afterRead != nil {
		boundary.afterRead(rel)
	}
	after, err := file.Stat()
	if err != nil || !doctorD9SameSnapshot(opened, after) {
		return nil, opened, errors.New("doctor D9 file identity changed during read")
	}
	if err := boundary.verifyOpenedPath(rel, after, true); err != nil {
		return nil, opened, err
	}
	return data, after, nil
}

func (*doctorD9OSBoundary) OpenRoot(string) error {
	return errors.New("doctor D9 root acquisition is forbidden")
}

func (*doctorD9OSBoundary) OpenDot() error {
	return errors.New("doctor D9 root-dot open is forbidden")
}

func (*doctorD9OSBoundary) Control() error {
	return errors.New("doctor D9 syscall control is forbidden")
}

func (*doctorD9OSBoundary) Flock() error {
	return errors.New("doctor D9 flock is forbidden")
}

func (*doctorD9OSBoundary) Fstatfs() error {
	return errors.New("doctor D9 fstatfs is forbidden")
}

func (*doctorD9OSBoundary) Unlock() error {
	return errors.New("doctor D9 unlock is forbidden")
}

func (*doctorD9OSBoundary) RunProcess(string, ...string) error {
	return errors.New("doctor D9 process execution is forbidden")
}

func (*doctorD9OSBoundary) Write(string, []byte) error {
	return errors.New("doctor D9 writes are forbidden")
}

func (boundary *doctorD9OSBoundary) nativeRel(rel string) (string, error) {
	if !doctorD9ReportPathSafe(rel) {
		return "", errors.New("doctor D9 unsafe relative path")
	}
	native := filepath.FromSlash(rel)
	absolute := filepath.Join(boundary.root, native)
	if err := safety.EnsureSafeRepoPath(boundary.root, absolute); err != nil {
		return "", err
	}
	return native, nil
}

func (boundary *doctorD9OSBoundary) inspectPath(rel string) (fs.FileInfo, error) {
	if _, err := boundary.nativeRel(rel); err != nil {
		return nil, err
	}
	parts := strings.Split(rel, "/")
	for index := 1; index <= len(parts); index++ {
		component := filepath.Join(boundary.root, filepath.FromSlash(strings.Join(parts[:index], "/")))
		info, err := os.Lstat(component)
		if err != nil {
			return nil, err
		}
		if index < len(parts) && (doctorD9RefusedInfo(info) || !info.IsDir()) {
			return nil, errors.New("doctor D9 ancestor boundary refused")
		}
		if index == len(parts) {
			return info, nil
		}
	}
	return nil, errors.New("doctor D9 empty path inspection")
}

func (boundary *doctorD9OSBoundary) openConfined(rel string) (*os.File, fs.FileInfo, error) {
	native, err := boundary.nativeRel(rel)
	if err != nil {
		return nil, nil, err
	}
	before, err := boundary.inspectPath(rel)
	if err != nil {
		return nil, nil, err
	}
	if doctorD9RefusedInfo(before) {
		return nil, before, errors.New("doctor D9 path boundary refused")
	}
	if boundary.beforeOpen != nil {
		boundary.beforeOpen(rel)
	}
	file, err := os.OpenInRoot(boundary.root, native)
	if err != nil {
		return nil, before, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, before, err
	}
	if boundary.afterOpen != nil {
		boundary.afterOpen(rel)
	}
	if !doctorD9SameObject(before, opened) {
		file.Close()
		return nil, before, errors.New("doctor D9 opened path identity changed")
	}
	if err := boundary.verifyOpenedPath(rel, opened, false); err != nil {
		file.Close()
		return nil, before, err
	}
	return file, opened, nil
}

func (boundary *doctorD9OSBoundary) verifyOpenedPath(rel string, opened fs.FileInfo, snapshot bool) error {
	current, err := boundary.inspectPath(rel)
	if err != nil || doctorD9RefusedInfo(current) || !doctorD9SameObject(opened, current) {
		return errors.New("doctor D9 path identity changed")
	}
	if snapshot && !doctorD9SameSnapshot(opened, current) {
		return errors.New("doctor D9 path contents changed")
	}
	return nil
}

func doctorD9SameObject(left, right fs.FileInfo) bool {
	return left != nil && right != nil && os.SameFile(left, right) && left.Mode().Type() == right.Mode().Type()
}

func doctorD9SameSnapshot(left, right fs.FileInfo) bool {
	return doctorD9SameObject(left, right) &&
		left.Mode() == right.Mode() &&
		left.Size() == right.Size() &&
		left.ModTime().Equal(right.ModTime())
}

func runDoctorD9(ctx *doctorContext) {
	boundary := newDoctorD9Boundary(ctx.root)
	runDoctorD9PrepareEvidence(ctx, boundary)
	runDoctorD9ArchiveEvidence(ctx, boundary)
}

func runDoctorD9PrepareEvidence(ctx *doctorContext, boundary doctorD9Boundary) {
	info, err := boundary.Lstat(doctorD9PrepareLaneRel)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil || doctorD9RefusedInfo(info) || !info.IsDir() {
		doctorD9AddUnsafeFinding(ctx, "", doctorD9PrepareLaneRel,
			"Prepare evidence could not be inspected through the ordinary doctor read boundary.",
			"Run tpatch doctor --check D9 after correcting the unsafe transaction-lane entry.")
		return
	}
	lanes, err := boundary.ReadDir(doctorD9PrepareLaneRel)
	if err != nil {
		doctorD9AddUnsafeFinding(ctx, "", doctorD9PrepareLaneRel,
			"Prepare evidence could not be enumerated through the ordinary doctor read boundary.",
			"Run tpatch doctor --check D9 after correcting the unsafe transaction-lane entry.")
		return
	}
	for _, lane := range lanes {
		slug, canonicalErr := intent.CanonicalSlug(lane.Name())
		if canonicalErr != nil || slug != lane.Name() || !doctorD9PathSegmentSafe(lane.Name()) {
			doctorD9AddUnsafeFinding(ctx, "", doctorD9PrepareLaneRel,
				"An unsafe transaction-lane entry was not inspected or echoed.",
				"Run tpatch doctor --check D9 after removing or renaming the unsafe entry with trusted filesystem tooling.")
			continue
		}
		laneRel := doctorD9PrepareLaneRel + "/" + slug
		laneInfo, statErr := boundary.Lstat(laneRel)
		if statErr != nil || doctorD9RefusedInfo(laneInfo) || !laneInfo.IsDir() {
			doctorD9AddUnsafeFinding(ctx, slug, laneRel,
				"The feature transaction lane is not a safe directory and was not traversed.",
				"Run tpatch prepare "+slug+" --abandon-transaction to inspect the evidence-preserving route.")
			continue
		}
		runDoctorD9PrepareLane(ctx, boundary, slug, laneRel)
	}
}

func runDoctorD9PrepareLane(ctx *doctorContext, boundary doctorD9Boundary, slug, laneRel string) {
	entries, err := boundary.ReadDir(laneRel)
	if err != nil {
		doctorD9AddUnsafeFinding(ctx, slug, laneRel,
			"The feature transaction lane could not be enumerated safely.",
			"Run tpatch prepare "+slug+" --abandon-transaction to inspect the evidence-preserving route.")
		return
	}
	armed := false
	for _, entry := range entries {
		if entry.Name() == "journal.json" || entry.Name() == "journal.clearing.json" {
			armed = true
		}
	}
	for _, entry := range entries {
		name := entry.Name()
		if !doctorD9PathSegmentSafe(name) {
			doctorD9AddUnsafeFinding(ctx, slug, laneRel,
				"An unsafe transaction-lane entry was not inspected or echoed.",
				"Run tpatch prepare "+slug+" --abandon-transaction to inspect the evidence-preserving route.")
			continue
		}
		rel := laneRel + "/" + name
		switch {
		case name == "journal.json" || name == "journal.clearing.json":
			if !doctorD9ExpectedKind(boundary, rel, false) {
				doctorD9AddUnsafeFinding(ctx, slug, rel,
					"Prepare control evidence is not a regular file and was not opened.",
					"Run tpatch prepare "+slug+" --abandon-transaction to inspect the evidence-preserving route.")
				continue
			}
			doctorD9AddFinding(ctx, DoctorFinding{
				CheckID:     "D9",
				Code:        "prepare-transaction-pending",
				Severity:    "warning",
				Feature:     slug,
				Path:        rel,
				Message:     "Durable prepare transaction control evidence is present; D9 does not inspect or make any claim about a live authority holder.",
				Fixable:     false,
				Remediation: "run tpatch prepare " + slug,
			})
		case doctorD9AbandonedName(name):
			if !doctorD9ExpectedKind(boundary, rel, true) {
				doctorD9AddUnsafeFinding(ctx, slug, rel,
					"Abandoned prepare evidence is not a safe directory and was not traversed.",
					"Run tpatch doctor --check D9 after correcting the unsafe abandoned-evidence entry.")
				continue
			}
			doctorD9AddFinding(ctx, DoctorFinding{
				CheckID:     "D9",
				Code:        "prepare-abandoned-evidence",
				Severity:    "warning",
				Feature:     slug,
				Path:        rel,
				Message:     "Previously abandoned prepare evidence is retained and is never removed by automatic recovery.",
				Fixable:     false,
				Remediation: "rm -rf -- " + doctorD9ShellQuote(rel),
			})
		case !armed && doctorD9StageName(name):
			if !doctorD9ExpectedKind(boundary, rel, true) {
				doctorD9AddUnsafeFinding(ctx, slug, rel,
					"Stale prepare staging evidence is not a safe directory and was not traversed.",
					"run tpatch prepare "+slug)
				continue
			}
			doctorD9AddFinding(ctx, DoctorFinding{
				CheckID:     "D9",
				Code:        "prepare-stage-stale",
				Severity:    "warning",
				Feature:     slug,
				Path:        rel,
				Message:     "Unarmed prepare staging residue is present; the next successful mutating prepare removes owned stale stages.",
				Fixable:     false,
				Remediation: "run tpatch prepare " + slug,
			})
		case !armed && (name == "index.preimage.json" || name == "status.preimage.json"):
			if !doctorD9ExpectedKind(boundary, rel, false) {
				doctorD9AddUnsafeFinding(ctx, slug, rel,
					"Unarmed prepare preimage evidence is not a regular file and was not opened.",
					"run tpatch prepare "+slug)
				continue
			}
			doctorD9AddFinding(ctx, DoctorFinding{
				CheckID:     "D9",
				Code:        "prepare-preimage-stale",
				Severity:    "warning",
				Feature:     slug,
				Path:        rel,
				Message:     "Unarmed prepare preimage residue is present; the next successful mutating prepare removes owned stale preimages.",
				Fixable:     false,
				Remediation: "run tpatch prepare " + slug,
			})
		case !armed && doctorD9LaneTempName(name):
			if !doctorD9ExpectedKind(boundary, rel, false) {
				doctorD9AddUnsafeFinding(ctx, slug, rel,
					"Unarmed prepare temporary evidence is not a regular file and was not opened.",
					"run tpatch prepare "+slug)
				continue
			}
			doctorD9AddFinding(ctx, DoctorFinding{
				CheckID:     "D9",
				Code:        "prepare-temp-stale",
				Severity:    "warning",
				Feature:     slug,
				Path:        rel,
				Message:     "Unarmed prepare control-write residue is present; the next successful mutating prepare removes owned stale temporary files.",
				Fixable:     false,
				Remediation: "run tpatch prepare " + slug,
			})
		}
	}
}

func runDoctorD9ArchiveEvidence(ctx *doctorContext, boundary doctorD9Boundary) {
	features, err := boundary.ReadDir(doctorD9FeaturesRel)
	if err != nil {
		doctorD9AddUnsafeFinding(ctx, "", doctorD9FeaturesRel,
			"Archive evidence could not be enumerated through the ordinary doctor read boundary.",
			"Run tpatch doctor --check D9 after correcting the unsafe feature entry.")
		return
	}
	for _, feature := range features {
		slug, canonicalErr := intent.CanonicalSlug(feature.Name())
		if canonicalErr != nil || slug != feature.Name() || !doctorD9PathSegmentSafe(feature.Name()) {
			doctorD9AddUnsafeFinding(ctx, "", doctorD9FeaturesRel,
				"An unsafe feature entry was not inspected or echoed for archive evidence.",
				"Run tpatch doctor --check D9 after removing or renaming the unsafe entry with trusted filesystem tooling.")
			continue
		}
		featureRel := doctorD9FeaturesRel + "/" + slug
		featureInfo, statErr := boundary.Lstat(featureRel)
		if statErr != nil || doctorD9RefusedInfo(featureInfo) || !featureInfo.IsDir() {
			doctorD9AddUnsafeFinding(ctx, slug, featureRel,
				"The feature archive boundary is not a safe directory and was not traversed.",
				"Run tpatch doctor --check D9 after correcting the unsafe feature entry.")
			continue
		}
		archiveRootRel, relErr := store.IntentArchiveRootRel(slug)
		if relErr != nil {
			continue
		}
		archiveInfo, archiveErr := boundary.Lstat(archiveRootRel)
		if errors.Is(archiveErr, fs.ErrNotExist) {
			continue
		}
		if archiveErr != nil || doctorD9RefusedInfo(archiveInfo) || !archiveInfo.IsDir() {
			doctorD9AddUnsafeFinding(ctx, slug, archiveRootRel,
				"The intent archive is not a safe directory and was not traversed.",
				"run tpatch feature intent-archive list "+slug)
			continue
		}
		runDoctorD9FeatureArchive(ctx, boundary, slug)
	}
}

func runDoctorD9FeatureArchive(ctx *doctorContext, boundary doctorD9Boundary, slug string) {
	storage := &doctorD9ArchiveStorage{boundary: boundary, feature: slug}
	snapshot, err := store.CaptureIntentArchive(storage, slug)
	if err != nil {
		if storage.unavailable {
			if hashes, ok := doctorD9PendingHashesFromValidatedIndex(snapshot); ok {
				doctorD9ReportPendingHashes(ctx, slug, hashes)
			}
			reportPath := storage.failurePath
			if !doctorD9ReportPathSafe(reportPath) {
				reportPath, _ = store.IntentArchiveRootRel(slug)
			}
			doctorD9AddUnsafeFinding(ctx, slug, reportPath,
				"Persistent archive evidence was unavailable or changed identity during inspection; D9 did not classify it as corrupt and offers no destructive repair.",
				"run tpatch feature intent-archive list "+slug)
			return
		}
		var archiveErr *store.IntentArchiveError
		code := string(store.IntentArchiveCodeStorageFailed)
		if errors.As(err, &archiveErr) && archiveErr.Code != "" {
			code = string(archiveErr.Code)
		}
		indexRel, _ := store.IntentArchiveIndexRel(slug)
		reportPath := storage.failurePath
		if !doctorD9ReportPathSafe(reportPath) {
			reportPath = indexRel
		}
		doctorD9AddFinding(ctx, DoctorFinding{
			CheckID:     "D9",
			Code:        "archive-index-invalid",
			Severity:    "warning",
			Feature:     slug,
			Tag:         "index",
			Path:        reportPath,
			Message:     "Persistent archive evidence failed strict validation with bind code " + code + "; D9 did not echo archive contents or unsafe paths.",
			Fixable:     false,
			Remediation: "run tpatch feature intent-archive list " + slug,
		})
		return
	}
	doctorD9ReportPendingArchive(ctx, snapshot)
	doctorD9ReportArchiveClasses(ctx, snapshot)
}

func doctorD9ReportPendingArchive(ctx *doctorContext, snapshot store.IntentArchiveSnapshot) {
	doctorD9ReportPendingHashes(ctx, snapshot.Feature, snapshot.Inspection.PendingHashes)
}

func doctorD9PendingHashesFromValidatedIndex(snapshot store.IntentArchiveSnapshot) ([]string, bool) {
	if !snapshot.IndexCapture.Exists || snapshot.Feature == "" {
		return nil, false
	}
	if err := store.ValidateIntentArchiveIndex(snapshot.Index, snapshot.Feature); err != nil {
		return nil, false
	}
	return store.PendingIntentArchiveHashes(snapshot.Index), true
}

func doctorD9ReportPendingHashes(ctx *doctorContext, feature string, pending []string) {
	hashes := doctorD9SortedUnique(pending)
	if len(hashes) == 0 {
		return
	}
	retry := doctorD9BlobPurgeCommand(feature, hashes)
	indexRel, _ := store.IntentArchiveIndexRel(feature)
	instances := make([]string, 0, len(hashes))
	for _, hash := range hashes {
		blobRel, _ := store.IntentArchiveBlobRel(feature, hash)
		instances = append(instances, blobRel)
	}
	reportPath := ""
	if len(hashes) == 1 {
		reportPath, _ = store.IntentArchiveBlobRel(feature, hashes[0])
	}
	doctorD9AddFinding(ctx, DoctorFinding{
		CheckID:     "D9",
		Code:        "archive-purge-pending",
		Severity:    "warning",
		Feature:     feature,
		Tag:         "pending-purge",
		Path:        reportPath,
		Message:     fmt.Sprintf("Archive purge ownership is durable for %d hash instance(s): %s. The index evidence is %s.", len(hashes), strings.Join(instances, ", "), indexRel),
		Fixable:     false,
		Remediation: retry,
	})
}

func doctorD9ReportArchiveClasses(ctx *doctorContext, snapshot store.IntentArchiveSnapshot) {
	for _, class := range snapshot.Inspection.Classes {
		code, message, reportPath, ok := doctorD9ArchiveClassFinding(class)
		if !ok {
			archiveRootRel, _ := store.IntentArchiveRootRel(snapshot.Feature)
			doctorD9AddUnsafeFinding(ctx, snapshot.Feature, archiveRootRel,
				"Persistent archive evidence contained an unsafe repair-class instance and was not echoed.",
				"run tpatch feature intent-archive list "+snapshot.Feature)
			continue
		}
		remediation := doctorD9ArchiveClassRemediation(snapshot.Feature, class)
		if len(snapshot.Inspection.Classes) > 1 {
			message += fmt.Sprintf(
				" Repair work is staged; this observed class has rank %d. Complete one stage, then rerun D9 to inventory the remaining stages.",
				class.Rank,
			)
		}
		doctorD9AddFinding(ctx, DoctorFinding{
			CheckID:     "D9",
			Code:        code,
			Severity:    "warning",
			Feature:     snapshot.Feature,
			Tag:         string(class.Class),
			Path:        reportPath,
			Message:     message,
			Fixable:     false,
			Remediation: remediation,
		})
	}
}

func doctorD9ArchiveClassFinding(class store.IntentArchiveRepairClassReport) (string, string, string, bool) {
	instances := make([]string, 0, len(class.Instances))
	for _, instance := range class.Instances {
		if !doctorD9ReportPathSafe(instance.Path) ||
			(instance.Hash != "" && !doctorD9ValidHex(instance.Hash, 64)) {
			return "", "", "", false
		}
		instances = append(instances, instance.Path)
	}
	instances = doctorD9SortedUnique(instances)
	reportPath := ""
	if len(instances) == 1 {
		reportPath = class.Instances[0].Path
	}
	count := len(instances)
	list := strings.Join(instances, ", ")
	switch class.Class {
	case store.IntentArchiveRepairCorruptObject:
		return string(store.IntentArchiveCodeBlobCorrupt),
			fmt.Sprintf("Managed archive object evidence is unidentifiable (%d corrupt-object instance(s)): %s. This rank-1 class blocks every tpatch repair selector in the archive until every listed removal has completed.", count, list),
			reportPath, true
	case store.IntentArchiveRepairDanglingReference:
		return string(store.IntentArchiveCodeBlobDangling),
			fmt.Sprintf("Retained archive references have no recoverable blob (%d dangling-reference instance(s)): %s.", count, list),
			reportPath, true
	case store.IntentArchiveRepairMixedReference:
		return string(store.IntentArchiveCodeIndexStorageInconsistent),
			fmt.Sprintf("Live and tombstoned archive references disagree for present hashes (%d mixed-reference instance(s)): %s.", count, list),
			reportPath, true
	case store.IntentArchiveRepairUnreferencedResidue:
		return "archive-orphan",
			fmt.Sprintf("The archive contains %d globally unreferenced blob residue instance(s): %s.", count, list),
			reportPath, true
	default:
		return "persistent-evidence-unsafe", "Persistent archive evidence could not be classified safely.", "", true
	}
}

func doctorD9ArchiveClassRemediation(slug string, class store.IntentArchiveRepairClassReport) string {
	switch class.Class {
	case store.IntentArchiveRepairCorruptObject:
		lines := []string{
			"WARNING: destructive archive repair. Remove every corrupt-object instance before running any tpatch repair selector in this archive; stop first if an object must be preserved with tooling appropriate to its type.",
		}
		for _, rel := range doctorD9SortedUnique(class.Paths) {
			lines = append(lines, "rm -rf -- "+doctorD9ShellQuote(rel))
		}
		hashes := []string{}
		for _, instance := range class.Instances {
			if instance.Hash != "" && doctorD9ContainsRepairClass(instance.ResultingClasses, store.IntentArchiveRepairDanglingReference) {
				hashes = append(hashes, instance.Hash)
			}
		}
		if len(hashes) != 0 {
			lines = append(lines,
				"After every removal above, run "+doctorD9BlobPurgeCommand(slug, hashes)+". Restoring each exact hash-correct object instead also resolves its observation.")
		} else {
			lines = append(lines, "Restoring each exact hash-correct object instead also resolves its observation.")
		}
		lines = append(lines, "A committed blob remains in this repository's Git history; tpatch does not rewrite Git history.")
		return strings.Join(lines, "\n")
	case store.IntentArchiveRepairDanglingReference, store.IntentArchiveRepairMixedReference:
		return doctorD9BlobPurgeCommand(slug, doctorD9SortedUnique(class.Hashes))
	case store.IntentArchiveRepairUnreferencedResidue:
		return "tpatch feature intent-archive purge " + slug + " --orphans --yes"
	default:
		return "run tpatch feature intent-archive list " + slug
	}
}

type doctorD9ArchiveStorage struct {
	boundary    doctorD9Boundary
	feature     string
	failurePath string
	unavailable bool
}

func (storage *doctorD9ArchiveStorage) CaptureIndex(indexRel string) (store.IntentArchiveIndexCapture, error) {
	storage.failurePath = indexRel
	info, err := storage.boundary.Lstat(indexRel)
	if errors.Is(err, fs.ErrNotExist) {
		return store.IntentArchiveIndexCapture{
			Exists:   false,
			Raw:      []byte{},
			Identity: "absent",
		}, nil
	}
	if err != nil || doctorD9RefusedInfo(info) || !info.Mode().IsRegular() {
		storage.unavailable = true
		return store.IntentArchiveIndexCapture{}, &store.IntentArchiveError{
			Code:      store.IntentArchiveCodeStorageFailed,
			Class:     "index-read-boundary",
			ExitClass: 3,
		}
	}
	data, opened, err := storage.boundary.ReadRegular(indexRel, intent.MaxArtifactBytes)
	if err != nil {
		storage.unavailable = true
		return store.IntentArchiveIndexCapture{}, &store.IntentArchiveError{
			Code:      store.IntentArchiveCodeStorageFailed,
			Class:     "index-read-boundary",
			ExitClass: 3,
		}
	}
	return store.IntentArchiveIndexCapture{
		Exists:   true,
		Raw:      data,
		Identity: doctorD9Identity(data, opened.Mode()),
	}, nil
}

func (storage *doctorD9ArchiveStorage) EnumerateBlobs(blobsRel string) ([]string, error) {
	storage.failurePath = blobsRel
	info, err := storage.boundary.Lstat(blobsRel)
	if errors.Is(err, fs.ErrNotExist) {
		return []string{}, nil
	}
	if err != nil || doctorD9RefusedInfo(info) || !info.IsDir() {
		storage.unavailable = true
		return nil, &store.IntentArchiveError{
			Code:      store.IntentArchiveCodeStorageFailed,
			Class:     "blobs-directory-boundary",
			ExitClass: 3,
		}
	}
	entries, err := storage.boundary.ReadDir(blobsRel)
	if err != nil {
		storage.unavailable = true
		return nil, &store.IntentArchiveError{
			Code:      store.IntentArchiveCodeStorageFailed,
			Class:     "blobs-directory-boundary",
			ExitClass: 3,
		}
	}
	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !doctorD9PathSegmentSafe(entry.Name()) {
			return nil, &store.IntentArchiveError{
				Code:      store.IntentArchiveCodeIndexPathEscape,
				Class:     "unsafe-blob-entry",
				ExitClass: 3,
			}
		}
		rel := blobsRel + "/" + entry.Name()
		if !doctorD9ManagedBlobPathSafe(storage.feature, rel) {
			return nil, &store.IntentArchiveError{
				Code:      store.IntentArchiveCodeIndexPathEscape,
				Class:     "unsafe-blob-entry",
				ExitClass: 3,
			}
		}
		out = append(out, rel)
	}
	sort.Strings(out)
	return out, nil
}

func (storage *doctorD9ArchiveStorage) ProbeBlob(blobRel string) (store.IntentArchiveBlobProbe, error) {
	storage.failurePath = blobRel
	if !doctorD9ManagedBlobPathSafe(storage.feature, blobRel) {
		storage.failurePath, _ = store.IntentArchiveBlobsRel(storage.feature)
		return store.IntentArchiveBlobProbe{}, &store.IntentArchiveError{
			Code:      store.IntentArchiveCodeIndexPathEscape,
			Class:     "unsafe-blob-path",
			ExitClass: 3,
		}
	}
	info, err := storage.boundary.Lstat(blobRel)
	if errors.Is(err, fs.ErrNotExist) {
		return store.IntentArchiveBlobProbe{Kind: store.IntentArchiveBlobKindAbsent, Identity: "absent"}, nil
	}
	if err != nil {
		storage.unavailable = true
		return store.IntentArchiveBlobProbe{}, &store.IntentArchiveError{
			Code:      store.IntentArchiveCodeStorageFailed,
			Class:     "blob-read-boundary",
			ExitClass: 3,
		}
	}
	probe := store.IntentArchiveBlobProbe{
		SizeBytes: info.Size(),
		Identity:  store.IntentArchiveIdentityToken("object:" + strconv.FormatUint(uint64(info.Mode()), 10) + ":" + strconv.FormatInt(info.Size(), 10)),
	}
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		probe.Kind = store.IntentArchiveBlobKindSymlink
	case info.IsDir():
		probe.Kind = store.IntentArchiveBlobKindDirectory
	case info.Mode()&os.ModeNamedPipe != 0:
		probe.Kind = store.IntentArchiveBlobKindFIFO
	case info.Mode()&os.ModeDevice != 0:
		probe.Kind = store.IntentArchiveBlobKindDevice
	case !info.Mode().IsRegular() || info.Mode()&os.ModeIrregular != 0:
		probe.Kind = store.IntentArchiveBlobKindOtherNonRegular
	default:
		data, opened, readErr := storage.boundary.ReadRegular(blobRel, intent.MaxArtifactBytes)
		if readErr != nil {
			storage.unavailable = true
			return store.IntentArchiveBlobProbe{}, &store.IntentArchiveError{
				Code:      store.IntentArchiveCodeStorageFailed,
				Class:     "blob-read-boundary",
				ExitClass: 3,
			}
		}
		sum := sha256.Sum256(data)
		probe.Kind = store.IntentArchiveBlobKindRegular
		probe.SHA256 = hex.EncodeToString(sum[:])
		probe.SizeBytes = int64(len(data))
		probe.Identity = doctorD9Identity(data, opened.Mode())
	}
	return probe, nil
}

func (*doctorD9ArchiveStorage) PreflightIndexCAS(string, store.IntentArchiveIdentityToken) error {
	return errors.New("doctor D9 archive mutation is forbidden")
}

func (*doctorD9ArchiveStorage) PreflightBlobRemove(string, store.IntentArchiveIdentityToken) error {
	return errors.New("doctor D9 archive mutation is forbidden")
}

func (*doctorD9ArchiveStorage) PublishBlob(string, string, []byte) (store.IntentArchiveMutationResult, error) {
	return store.IntentArchiveMutationResult{}, errors.New("doctor D9 archive mutation is forbidden")
}

func (*doctorD9ArchiveStorage) CASIndex(string, store.IntentArchiveIdentityToken, []byte) (store.IntentArchiveMutationResult, error) {
	return store.IntentArchiveMutationResult{}, errors.New("doctor D9 archive mutation is forbidden")
}

func (*doctorD9ArchiveStorage) RemoveBlob(string, store.IntentArchiveIdentityToken) (store.IntentArchiveMutationResult, error) {
	return store.IntentArchiveMutationResult{}, errors.New("doctor D9 archive mutation is forbidden")
}

func (*doctorD9ArchiveStorage) SyncDirectory(string) error {
	return errors.New("doctor D9 archive mutation is forbidden")
}

func doctorD9AddUnsafeFinding(ctx *doctorContext, slug, rel, message, remediation string) {
	if !doctorD9ReportPathSafe(rel) {
		rel = ".tpatch"
	}
	doctorD9AddFinding(ctx, DoctorFinding{
		CheckID:     "D9",
		Code:        "persistent-evidence-unsafe",
		Severity:    "warning",
		Feature:     slug,
		Tag:         "unsafe",
		Path:        rel,
		Message:     message,
		Fixable:     false,
		Remediation: remediation,
	})
}

func doctorD9AddFinding(ctx *doctorContext, finding DoctorFinding) {
	for _, known := range doctorD9PersistentEvidenceCodes {
		if finding.Code == known {
			ctx.addFinding(finding)
			return
		}
	}
	panic("doctor D9 emitted an unregistered persistent-evidence class")
}

func doctorD9ExpectedKind(boundary doctorD9Boundary, rel string, directory bool) bool {
	info, err := boundary.Lstat(rel)
	if err != nil || doctorD9RefusedInfo(info) {
		return false
	}
	if directory {
		return info.IsDir()
	}
	return info.Mode().IsRegular()
}

func doctorD9RefusedInfo(info fs.FileInfo) bool {
	return info == nil || info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0
}

func doctorD9PathSegmentSafe(value string) bool {
	if value == "" || value == "." || value == ".." || strings.Contains(value, "/") || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func doctorD9ReportPathSafe(value string) bool {
	if value == "" || value == "." || !fs.ValidPath(value) || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func doctorD9ManagedBlobPathSafe(slug, value string) bool {
	if !doctorD9ReportPathSafe(value) {
		return false
	}
	blobsRel, err := store.IntentArchiveBlobsRel(slug)
	return err == nil && path.Dir(value) == blobsRel
}

func doctorD9StageName(name string) bool {
	return strings.HasPrefix(name, "stage-") && doctorD9ValidHex(strings.TrimPrefix(name, "stage-"), 12)
}

func doctorD9AbandonedName(name string) bool {
	return strings.HasPrefix(name, "abandoned-") && doctorD9ValidHex(strings.TrimPrefix(name, "abandoned-"), 12)
}

func doctorD9LaneTempName(name string) bool {
	for _, base := range []string{
		"journal.json",
		"journal.clearing.json",
		"index.preimage.json",
		"status.preimage.json",
	} {
		prefix := "." + base + ".tmp-"
		if strings.HasPrefix(name, prefix) && doctorD9ValidHex(strings.TrimPrefix(name, prefix), 12) {
			return true
		}
	}
	return false
}

func doctorD9ValidHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	for _, character := range value {
		if (character < '0' || character > '9') && (character < 'a' || character > 'f') {
			return false
		}
	}
	return true
}

func doctorD9BlobPurgeCommand(slug string, hashes []string) string {
	unique := doctorD9SortedUnique(hashes)
	var command strings.Builder
	command.WriteString("tpatch feature intent-archive purge ")
	command.WriteString(slug)
	for _, hash := range unique {
		command.WriteString(" --blob ")
		command.WriteString(hash)
	}
	command.WriteString(" --yes")
	return command.String()
}

func doctorD9SortedUnique(values []string) []string {
	sorted := append([]string(nil), values...)
	sort.Strings(sorted)
	unique := sorted[:0]
	for _, value := range sorted {
		if len(unique) == 0 || unique[len(unique)-1] != value {
			unique = append(unique, value)
		}
	}
	return unique
}

func doctorD9ContainsRepairClass(classes []store.IntentArchiveRepairClass, want store.IntentArchiveRepairClass) bool {
	for _, class := range classes {
		if class == want {
			return true
		}
	}
	return false
}

func doctorD9Identity(data []byte, mode fs.FileMode) store.IntentArchiveIdentityToken {
	sum := sha256.Sum256(data)
	return store.IntentArchiveIdentityToken(
		"file:" + hex.EncodeToString(sum[:]) + ":" +
			strconv.FormatInt(int64(len(data)), 10) + ":" +
			strconv.FormatUint(uint64(mode), 10),
	)
}

func doctorD9ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}
