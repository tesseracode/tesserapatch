package workflow

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tesseracode/tesserapatch/internal/safety"
)

const (
	doctorD6ChangelogPath      = "CHANGELOG.md"
	doctorD6ChangelogRemediate = "follow RELEASING.md Step 1 — Write the CHANGELOG.md entry"
	doctorD6TagRemediate       = "follow RELEASING.md Step 2 — Tag the release commit"
	doctorD6GHReleaseRemediate = "follow RELEASING.md Step 3 — Publish the GitHub Release"
	doctorD6MetadataRemediate  = "provide a local release snapshot from: gh release list --json tagName,url,publishedAt"
)

var (
	doctorReleaseTagRe       = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	doctorChangelogReleaseRe = regexp.MustCompile(`^## (v[0-9]+\.[0-9]+\.[0-9]+)(?:\s|$)`)
)

type doctorReleaseSnapshot struct {
	Tags map[string]bool
}

type doctorReleaseMetadataFile struct {
	Releases []doctorReleaseMetadataEntry `json:"releases"`
}

type doctorReleaseMetadataEntry struct {
	Tag              string `json:"tag"`
	TagName          string `json:"tagName"`
	URL              string `json:"url"`
	PublishedAt      string `json:"publishedAt"`
	PublishedAtSnake string `json:"published_at"`
}

func runDoctorD6(ctx *doctorContext) {
	if _, err := os.Stat(filepath.Join(ctx.root, ".git")); err != nil {
		if os.IsNotExist(err) {
			return
		}
		ctx.addFinding(DoctorFinding{
			CheckID:  "D6",
			Code:     "release-git-metadata-unreadable",
			Severity: "error",
			Message:  fmt.Sprintf("cannot stat local git metadata: %v", err),
			Fixable:  false,
		})
		return
	}
	tags, err := doctorD6LocalReleaseTags(ctx.root)
	if err != nil {
		ctx.addFinding(DoctorFinding{
			CheckID:  "D6",
			Code:     "release-tags-unreadable",
			Severity: "error",
			Message:  fmt.Sprintf("cannot list local git tags: %v", err),
			Fixable:  false,
		})
		return
	}
	changelogPath := filepath.Join(ctx.root, doctorD6ChangelogPath)
	changelogReleases, err := doctorD6ChangelogReleases(changelogPath)
	if err != nil {
		ctx.addFinding(DoctorFinding{
			CheckID:     "D6",
			Code:        "release-changelog-unreadable",
			Severity:    "error",
			Path:        doctorD6ChangelogPath,
			Message:     fmt.Sprintf("cannot read CHANGELOG.md release headings: %v", err),
			Fixable:     false,
			Remediation: doctorD6ChangelogRemediate,
		})
		return
	}
	for _, tag := range sortedKeys(tags) {
		if !changelogReleases[tag] {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D6",
				Code:        "release-tag-missing-changelog",
				Severity:    "drift",
				Tag:         tag,
				Path:        doctorD6ChangelogPath,
				Message:     fmt.Sprintf("local release tag %s has no matching CHANGELOG.md release heading", tag),
				Fixable:     false,
				Remediation: doctorD6ChangelogRemediate,
			})
		}
	}
	for _, tag := range sortedKeys(changelogReleases) {
		if !tags[tag] {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D6",
				Code:        "release-changelog-missing-tag",
				Severity:    "drift",
				Tag:         tag,
				Path:        doctorD6ChangelogPath,
				Message:     fmt.Sprintf("CHANGELOG.md release heading %s has no matching local git tag", tag),
				Fixable:     false,
				Remediation: doctorD6TagRemediate,
			})
		}
	}
	if strings.TrimSpace(ctx.options.ReleaseMetadata) == "" {
		for _, tag := range sortedKeys(tags) {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D6",
				Code:        "release-gh-release-unknown",
				Severity:    "warning",
				Tag:         tag,
				Message:     fmt.Sprintf("GitHub Release status for %s is unknown because no --release-metadata local snapshot was provided; doctor does not contact the GitHub API or prompt for auth", tag),
				Fixable:     false,
				Remediation: doctorD6MetadataRemediate,
			})
		}
		return
	}
	snapshot, metadataPath, ok := doctorD6LoadReleaseSnapshot(ctx)
	if !ok {
		return
	}
	for _, tag := range sortedKeys(tags) {
		if !snapshot.Tags[tag] {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D6",
				Code:        "release-missing-gh-release",
				Severity:    "drift",
				Tag:         tag,
				Path:        relOrAbs(ctx.root, metadataPath),
				Message:     fmt.Sprintf("local release tag %s is absent from the provided --release-metadata snapshot", tag),
				Fixable:     false,
				Remediation: doctorD6GHReleaseRemediate,
			})
		}
	}
	for _, tag := range sortedKeys(snapshot.Tags) {
		if !tags[tag] {
			ctx.addFinding(DoctorFinding{
				CheckID:     "D6",
				Code:        "release-snapshot-missing-local-tag",
				Severity:    "drift",
				Tag:         tag,
				Path:        relOrAbs(ctx.root, metadataPath),
				Message:     fmt.Sprintf("--release-metadata snapshot contains %s but no matching local git tag exists", tag),
				Fixable:     false,
				Remediation: doctorD6TagRemediate,
			})
		}
	}
}

func doctorD6LocalReleaseTags(root string) (map[string]bool, error) {
	cmd := exec.Command("git", "tag", "-l")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	tags := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		tag := strings.TrimSpace(line)
		if doctorReleaseTagRe.MatchString(tag) {
			tags[tag] = true
		}
	}
	return tags, nil
}

func doctorD6ChangelogReleases(path string) (map[string]bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	releases := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "## ") || strings.Contains(line, "(unreleased)") {
			continue
		}
		match := doctorChangelogReleaseRe.FindStringSubmatch(line)
		if len(match) == 2 {
			releases[match[1]] = true
		}
	}
	return releases, nil
}

func doctorD6LoadReleaseSnapshot(ctx *doctorContext) (doctorReleaseSnapshot, string, bool) {
	path := ctx.options.ReleaseMetadata
	if !filepath.IsAbs(path) {
		path = filepath.Join(ctx.root, path)
	}
	if err := safety.EnsureSafeRepoPath(ctx.root, path); err != nil {
		ctx.addFinding(DoctorFinding{
			CheckID:     "D6",
			Code:        "release-metadata-unsafe-path",
			Severity:    "error",
			Path:        relOrAbs(ctx.root, path),
			Message:     fmt.Sprintf("--release-metadata path is outside the workspace: %v", err),
			Fixable:     false,
			Remediation: doctorD6MetadataRemediate,
		})
		return doctorReleaseSnapshot{}, path, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		ctx.addFinding(DoctorFinding{
			CheckID:     "D6",
			Code:        "release-metadata-unreadable",
			Severity:    "error",
			Path:        relOrAbs(ctx.root, path),
			Message:     fmt.Sprintf("cannot read --release-metadata local snapshot: %v", err),
			Fixable:     false,
			Remediation: doctorD6MetadataRemediate,
		})
		return doctorReleaseSnapshot{}, path, false
	}
	snapshot, err := doctorD6ParseReleaseMetadata(data)
	if err != nil {
		ctx.addFinding(DoctorFinding{
			CheckID:     "D6",
			Code:        "release-metadata-malformed",
			Severity:    "error",
			Path:        relOrAbs(ctx.root, path),
			Line:        lineForJSONErrorBytes(data, err),
			Message:     fmt.Sprintf("--release-metadata snapshot is malformed: %v", err),
			Fixable:     false,
			Remediation: doctorD6MetadataRemediate,
		})
		return doctorReleaseSnapshot{}, path, false
	}
	return snapshot, path, true
}

func doctorD6ParseReleaseMetadata(data []byte) (doctorReleaseSnapshot, error) {
	var entries []doctorReleaseMetadataEntry
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")) {
		var wrapped doctorReleaseMetadataFile
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&wrapped); err != nil {
			return doctorReleaseSnapshot{}, err
		}
		entries = wrapped.Releases
	} else {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&entries); err != nil {
			return doctorReleaseSnapshot{}, err
		}
	}
	tags := map[string]bool{}
	for i, entry := range entries {
		tag := strings.TrimSpace(entry.TagName)
		if tag == "" {
			tag = strings.TrimSpace(entry.Tag)
		}
		if tag == "" {
			return doctorReleaseSnapshot{}, fmt.Errorf("release entry %d missing tagName/tag", i+1)
		}
		if doctorReleaseTagRe.MatchString(tag) {
			tags[tag] = true
		}
	}
	return doctorReleaseSnapshot{Tags: tags}, nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
