package testutil

import (
	"os"
	"testing"
)

func TestPinGitAutoGCOffDisablesDetachedMaintenance(t *testing.T) {
	keys := []string{
		"GIT_CONFIG_COUNT",
		"GIT_CONFIG_KEY_0", "GIT_CONFIG_VALUE_0",
		"GIT_CONFIG_KEY_1", "GIT_CONFIG_VALUE_1",
		"GIT_CONFIG_KEY_2", "GIT_CONFIG_VALUE_2",
		"GIT_CONFIG_KEY_3", "GIT_CONFIG_VALUE_3",
	}
	for _, key := range keys {
		t.Setenv(key, "")
	}

	PinGitAutoGCOff()

	want := map[string]string{
		"GIT_CONFIG_COUNT":   "4",
		"GIT_CONFIG_KEY_0":   "gc.auto",
		"GIT_CONFIG_VALUE_0": "0",
		"GIT_CONFIG_KEY_1":   "gc.autoDetach",
		"GIT_CONFIG_VALUE_1": "false",
		"GIT_CONFIG_KEY_2":   "maintenance.auto",
		"GIT_CONFIG_VALUE_2": "false",
		"GIT_CONFIG_KEY_3":   "maintenance.autoDetach",
		"GIT_CONFIG_VALUE_3": "false",
	}
	for key, value := range want {
		if got := os.Getenv(key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}
}
