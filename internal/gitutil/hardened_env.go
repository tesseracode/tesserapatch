package gitutil

import (
	"os"
	"strings"
)

// HardenedNoLazyGitEnv builds a scrubbed git environment that pins locale,
// disables replacement objects, and refuses lazy fetches.
func HardenedNoLazyGitEnv(extra ...string) []string {
	env := make([]string, 0, len(os.Environ())+len(extra)+6)
	for _, entry := range os.Environ() {
		name, _, ok := strings.Cut(entry, "=")
		if !ok || scrubHardenedGitVariable(name) || name == "LC_ALL" || name == "LANG" {
			continue
		}
		env = append(env, entry)
	}
	env = append(env, extra...)
	env = append(env,
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_SYSTEM="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
		"GIT_NO_REPLACE_OBJECTS=1",
		NoLazyFetchEnv,
		CLocaleEnv,
		"LANG=C",
	)
	return env
}

func scrubHardenedGitVariable(name string) bool {
	switch name {
	case "GIT_DIR",
		"GIT_WORK_TREE",
		"GIT_INDEX_FILE",
		"GIT_COMMON_DIR",
		"GIT_CEILING_DIRECTORIES",
		"GIT_OBJECT_DIRECTORY",
		"GIT_ALTERNATE_OBJECT_DIRECTORIES",
		"GIT_DISCOVERY_ACROSS_FILESYSTEM",
		"GIT_PREFIX",
		"GIT_IMPLICIT_WORK_TREE",
		"GIT_SUPER_PREFIX",
		"GIT_REPLACE_REF_BASE",
		"GIT_CONFIG",
		"GIT_CONFIG_GLOBAL",
		"GIT_CONFIG_SYSTEM",
		"GIT_CONFIG_NOSYSTEM",
		"GIT_CONFIG_COUNT":
		return true
	}
	for _, prefix := range []string{"GIT_CONFIG_KEY_", "GIT_CONFIG_VALUE_"} {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		suffix := strings.TrimPrefix(name, prefix)
		if suffix == "" {
			return false
		}
		for idx := 0; idx < len(suffix); idx++ {
			if suffix[idx] < '0' || suffix[idx] > '9' {
				return false
			}
		}
		return true
	}
	return false
}
