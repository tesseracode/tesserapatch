package gitutil

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"
)

type captureConversionConfig struct {
	filters     map[string]bool
	diffDrivers map[string]bool
}

func parseCaptureConversionConfig(raw string) (captureConversionConfig, error) {
	config := captureConversionConfig{filters: map[string]bool{}, diffDrivers: map[string]bool{}}
	entries, err := captureNULFields(raw)
	if err != nil {
		return config, fmt.Errorf("readonly capture received invalid conversion configuration: %w", err)
	}
	for _, entry := range entries {
		// A valueless boolean setting is emitted as key<NUL>, not key<LF><NUL>.
		key, value, _ := strings.Cut(entry, "\n")
		if key == "" {
			return config, fmt.Errorf("readonly capture received malformed conversion configuration")
		}
		switch {
		case strings.HasPrefix(key, "filter."):
			end := strings.LastIndexByte(key, '.')
			if end <= len("filter.") {
				return config, fmt.Errorf("readonly capture cannot establish the configured filter name")
			}
			config.filters[key[len("filter."):end]] = true
		case key == "core.autocrlf":
			switch strings.ToLower(strings.TrimSpace(value)) {
			case "false", "0", "no", "off":
			default:
				return config, fmt.Errorf("readonly capture cannot establish exact bytes without configured conversion/copy behavior")
			}
		case key == "diff.renames":
			if strings.Contains(strings.ToLower(value), "cop") {
				return config, fmt.Errorf("readonly capture cannot establish exact bytes without configured conversion/copy behavior")
			}
		case key == "diff.external":
			return config, fmt.Errorf("readonly capture cannot invoke a configured external diff driver")
		case strings.HasPrefix(key, "diff.") && strings.HasSuffix(key, ".command"):
			config.diffDrivers[strings.TrimSuffix(strings.TrimPrefix(key, "diff."), ".command")] = true
		case strings.HasPrefix(key, "diff.") && strings.HasSuffix(key, ".textconv"):
			config.diffDrivers[strings.TrimSuffix(strings.TrimPrefix(key, "diff."), ".textconv")] = true
		default:
			return config, fmt.Errorf("readonly capture received an unexpected conversion setting")
		}
	}
	return config, nil
}

func captureNULFields(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	if !strings.HasSuffix(raw, "\x00") {
		return nil, fmt.Errorf("unterminated NUL-delimited metadata")
	}
	return strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00"), nil
}

func captureAttributeCandidates(tracked string, untracked []string) ([]string, error) {
	paths, err := captureNULFields(tracked)
	if err != nil {
		return nil, fmt.Errorf("readonly capture received invalid tracked paths: %w", err)
	}
	seen := map[string]bool{}
	for _, path := range append(paths, untracked...) {
		if path == "" || strings.ContainsRune(path, 0) {
			return nil, fmt.Errorf("readonly capture received an invalid candidate path")
		}
		seen[path] = true
	}
	candidates := make([]string, 0, len(seen))
	for path := range seen {
		candidates = append(candidates, path)
	}
	sort.Strings(candidates)
	return candidates, nil
}

type captureAttribute struct {
	path, name, value string
}

func readCaptureAttributes(root string, paths []string) ([]captureAttribute, error) {
	if len(paths) == 0 {
		return nil, nil
	}
	cmd := exec.Command("git", "check-attr", "-z", "--all", "--stdin")
	cmd.Dir = root
	cmd.Env = CaptureReadOnlyEnv()
	cmd.Stdin = strings.NewReader(strings.Join(paths, "\x00") + "\x00")
	raw, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("readonly capture cannot resolve candidate attributes: %w", err)
	}
	return parseCaptureAttributes(string(raw), paths)
}

func parseCaptureAttributes(raw string, paths []string) ([]captureAttribute, error) {
	fields, err := captureNULFields(raw)
	if err != nil || len(fields)%3 != 0 {
		return nil, fmt.Errorf("readonly capture received malformed attribute metadata")
	}
	candidates := map[string]bool{}
	for _, path := range paths {
		candidates[path] = true
	}
	seen := map[[2]string]bool{}
	attributes := make([]captureAttribute, 0, len(fields)/3)
	for i := 0; i < len(fields); i += 3 {
		path, name, value := fields[i], fields[i+1], fields[i+2]
		key := [2]string{path, name}
		if !candidates[path] || name == "" || seen[key] {
			return nil, fmt.Errorf("readonly capture received out-of-scope or duplicate attribute metadata")
		}
		seen[key] = true
		attributes = append(attributes, captureAttribute{path, name, value})
	}
	return attributes, nil
}

func validateCaptureAttributes(attributes []captureAttribute, config captureConversionConfig, untracked []string) error {
	newPaths := map[string]bool{}
	for _, path := range untracked {
		newPaths[path] = true
	}
	for _, attribute := range attributes {
		switch attribute.name {
		case "filter":
			// --all omits genuinely unspecified attributes. Its "unset"
			// spelling is ambiguous with a driver literally named unset.
			if attribute.value == "unset" && !config.filters["unset"] {
				continue
			}
			return fmt.Errorf("readonly capture cannot establish exact bytes with filter %q on %q", attribute.value, attribute.path)
		case "diff":
			if config.diffDrivers[attribute.value] {
				return fmt.Errorf("readonly capture cannot invoke diff driver %q on %q", attribute.value, attribute.path)
			}
		default:
			if newPaths[attribute.path] {
				return fmt.Errorf("readonly untracked capture cannot establish attribute-converted bytes for %q", attribute.path)
			}
		}
	}
	return nil
}
