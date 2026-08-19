package cli

import (
	"bytes"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/tesseracode/tesserapatch/internal/rescap"
	"github.com/tesseracode/tesserapatch/internal/store"
)

const (
	preparePIBGoldenDir       = "testdata/prepare-pib-goldens"
	preparePIBGoldenCount     = 51
	preparePIBRuntimeCount    = preparePIBGoldenCount - 1
	preparePIBGoldenBinEnv    = "TPATCH_PREPARE_PIB_GOLDEN_BIN"
	preparePIBGoldenRecordEnv = "TPATCH_RECORD_PREPARE_PIB_GOLDENS"
	preparePIBGoldenSHAEnv    = "TPATCH_PREPARE_PIB_GOLDEN_SHA"
	prepareCheckGoldenBinEnv  = "TPATCH_PREPARE_CHECK_GOLDEN_BIN"
	prepareCheckGoldenSHAEnv  = "TPATCH_PREPARE_CHECK_GOLDEN_SHA"
	preparePIBBaseline        = "95cab04c481201675bb42263110d4711111c8d6d"
	preparePIBWaveBase        = "3b579fc7243bf0d1b21605d3c87562226f1fd936"
	prepareCheckBaseline      = "cacaaf867ebde100b699e20d76010f92316afc72"
	acceptedRoutingBaseline   = "9a8c1d049bb973ccf377bd9f0fa67d7080d2d773"
	acceptedRoutingTip        = "cacaaf867ebde100b699e20d76010f92316afc72"
	routingFixtureCommit      = "2cbccf63529309bce17f181053816fadfdcb112a"
	routingReadmeCommit       = "36f23b38c6d80234ea2924ec3d7cf0d1d5087f29"
	preparePIBSlug            = "pib-golden"
	preparePIBOldTime         = "2000-01-02T03:04:05Z"
	preparePIBToolVersion     = "prepare-pib-golden"
	ignoredFileResourceID     = "res_894d140c8685"
	gitMetadataResourceID     = "res_84270e6992fd"
)

var preparePIBRows = map[string][]string{
	"PIB-186": {"phase-auto-analyze.txt", "phase-auto-define.txt", "phase-auto-explore.txt"},
	"PIB-198": {"check-ready-human.txt", "check-not-ready-human.txt", "check-abort-human.txt"},
	"PIB-199": {"check-ready-json.txt", "check-not-ready-json.txt", "check-abort-json.txt"},
	"PIB-200": {"check-"},
	"PIB-207": {"check-ready-pending-human.txt", "check-ready-pending-json.txt"},
	"PIB-208": {"next-"},
	"PIB-210": {"phase-auto-"},
	"PIB-211": {"phase-manual-"},
	"PIB-212": {"compat-"},
	"PIB-286": {"resource-"},
}

var prepareCheckGoldenFixtures = []string{
	"check-ready-human.txt",
	"check-ready-json.txt",
	"check-not-ready-human.txt",
	"check-not-ready-json.txt",
	"check-abort-human.txt",
	"check-abort-json.txt",
	"check-ready-pending-human.txt",
	"check-ready-pending-json.txt",
}

var routingGoldenSHA256 = map[string]string{
	"README.md":                                  "a6ba7f95b761463f33ac6329e15badf56d550346d5152ad78e16a4a5452071f3",
	"changed-apply-help.txt":                     "5a7727472287062bd33fe367e1a1132fc70a49f0bb892b5c198680bb5f79bbed",
	"cycle-final-state.txt":                      "c441f9736e8c954ccf897cdfef221e5e1b2f7fc672baf84561d6d83c060b74d3",
	"cycle-skip-execute-transcript.txt":          "7ac957230e4a31171b112305d12cb19c29ffdc7865734b381af2befb0c0250a3",
	"next-analyzed-harness-json.txt":             "bdda13ca7ea2df83e6ee7aa8ca660c598d9dc630960faa1543aa69be0845f4f7",
	"next-analyzed-text.txt":                     "58ad3f956c9ae6dc98027b258685bd6a08c384ee1ac012ae94062089f4ec6669",
	"next-apply-mode-prepare.txt":                "7093fac4709f2bfb8dd092361e73deff27e1b634f2b863d6499a287e9312e4ed",
	"next-defined-post-explore-harness-json.txt": "582f18715a4eaf0dc70569dc3156b4eb2cb380f48274d5cf3e4af03131d44867",
	"next-defined-post-explore-text.txt":         "dbe9a3303b2af67f70c975d9fb9599be6f8f4a028ece25eaa7706476f60d4ee4",
	"next-defined-pre-explore-harness-json.txt":  "dcafae22188a638aa6b310ee020f86c79e6459d4d62a392df5bea457c8404966",
	"next-defined-pre-explore-text.txt":          "c39cdcba240bd0a89b26a5cf1ccadf5f2588014eb4a8a6fc0b662992ae4767cf",
	"next-requested-harness-json.txt":            "fb6d4fb7b729fc99f3e8ef3e55ce55489293833cb3ac4ca53663e91d153be8e1",
	"next-requested-text.txt":                    "13bb0c58dc2a7681455831c0f79b123e2aa62ed72c67487a44888e5b3ac9d30f",
}

func TestPreparePIBProducerEvidenceSensitivities(t *testing.T) {
	expected := prepareCheckBaseline
	valid := map[string]string{"vcs.revision": expected, "vcs.modified": "false"}
	cases := map[string]struct {
		ack      string
		settings map[string]string
	}{
		"missing-ack":      {"", valid},
		"wrong-ack":        {preparePIBBaseline, valid},
		"missing-revision": {expected, map[string]string{"vcs.modified": "false"}},
		"wrong-revision":   {expected, map[string]string{"vcs.revision": preparePIBBaseline, "vcs.modified": "false"}},
		"modified-source":  {expected, map[string]string{"vcs.revision": expected, "vcs.modified": "true"}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if err := validatePreparePIBProducerEvidence(tc.ack, expected, tc.settings); err == nil {
				t.Fatal("invalid producer evidence was accepted")
			}
		})
	}
	if err := validatePreparePIBProducerEvidence(expected, expected, valid); err != nil {
		t.Fatalf("valid producer evidence refused: %v", err)
	}
	for name, bins := range map[string][2]string{
		"phase-baseline": {"/tmp/baseline", ""},
		"check-baseline": {"", "/tmp/check-baseline"},
	} {
		t.Run("comparison-"+name, func(t *testing.T) {
			if err := validatePreparePIBComparisonInputs(bins[0], bins[1]); err == nil {
				t.Fatal("comparison mode accepted a supplied baseline")
			}
		})
	}
}

func TestPrepareCheckFixtureOrderingRejectsLateProduction(t *testing.T) {
	if err := validatePrepareCheckFixtureOrdering([]string{
		"internal/cli/prepare_pib_golden_test.go",
		"internal/intentpub/publish.go",
	}); err == nil {
		t.Fatal("ordering guard accepted a mutating production path before the golden fixture")
	}
	touched := parsePrepareCheckTouchedPaths([]byte(
		"internal/intentpub/publish.go\x00" +
			"internal/intentpub/publish.go\x00" +
			"internal/cli/prepare_pib_golden_test.go\x00"))
	if err := validatePrepareCheckFixtureOrdering(touched); err == nil {
		t.Fatal("ordering guard accepted a production edit followed by a revert")
	}
}

var unsupportedLockSourceHistory = []string{
	"c66845aca98fa6f4d72828e895bd8fe6a529a84c",
	"bff5ef5ae30653c0420a396dd6cae34053ad28bd",
}

var acceptedCheckSourceProvenance = map[string]struct {
	SHA256        string
	History       []string
	FreezeCurrent bool
}{
	"assets/assets_test.go": {
		SHA256:  "75f2a0d194eb3354b5f8667767449c6f79144d168f95a05e730106863ad94b84",
		History: []string{"0440337e3b8c75425c09309e8a08c37aeb0109c4"},
	},
	"assets/avp_parity_test.go": {
		SHA256:  "6ed90dfb0f373236337f8de399f5c87a4e9514467ff84eb57b9c0075e56703e8",
		History: []string{routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/cli/land_rev1_fold_test.go": {
		SHA256:  "aae8524fa3600a10a99a852ba44803852602b629cc7ca7df8b14c37ce84b0aff",
		History: []string{"54ab8b4d4253638d52a8d03de2e7b31b3ae2b2da"}, FreezeCurrent: true,
	},
	"internal/cli/prepare_test.go": {
		SHA256: "6bbb2b6a68fc167196e0721a0c26b8179d9c92ec90c0e1462b20761bd1db95fd",
		History: []string{
			routingFixtureCommit,
			"0440337e3b8c75425c09309e8a08c37aeb0109c4",
		},
	},
	"internal/cli/prepare_avp_test.go": {
		SHA256:  "539bd5ea11e55fd7f6fe00383c0b4c12674eff758e47c306545200bef8c114ce",
		History: []string{routingFixtureCommit},
	},
	"internal/cli/prepare_avp2_test.go": {
		SHA256: "e770a608d5a0930c3ef6edcaf2cf42d53c152d2273d82fe587f21fe926edb297",
		History: []string{
			"9b8efc57f5b77f557e524c3326b4397ba006e19c",
			"54ab8b4d4253638d52a8d03de2e7b31b3ae2b2da",
			routingFixtureCommit,
		},
	},
	"internal/cli/prepare_routing_golden_test.go": {
		SHA256: "372459f1743308786cd396c0205a98952eeb25f1e91cede5031a06fff1efba8c",
		History: []string{
			routingReadmeCommit,
			routingFixtureCommit,
		},
		FreezeCurrent: true,
	},
	"internal/intent/avp_classification_test.go": {
		SHA256:  "72910c2a9340c8d8d9aa495318d8d6d5b9d57c2ec7e8e5b788257174423797a4",
		History: []string{routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/intent/avp_document_guards_test.go": {
		SHA256:  "01d4e3759b9969949f6e1626c2d0fece4e056d4456de0b199300c9a7bb189cb0",
		History: []string{routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/intent/avp_guard_helpers_test.go": {
		SHA256: "10b46fac07b6993991ed35ffabbfcb016a1b6464afaead826cab101efa204c83",
		History: []string{
			"9b8efc57f5b77f557e524c3326b4397ba006e19c",
			"54ab8b4d4253638d52a8d03de2e7b31b3ae2b2da",
			"69dfe7c48a763667bad7815f679be9110c8322f3",
			routingReadmeCommit,
			routingFixtureCommit,
		},
		FreezeCurrent: true,
	},
	"internal/intent/avp_guards_test.go": {
		SHA256: "60e7a73661c22437c5d764cf1df7e9e1c96133a6b060ffe3701999701c941c38",
		History: []string{
			"9b8efc57f5b77f557e524c3326b4397ba006e19c",
			"54ab8b4d4253638d52a8d03de2e7b31b3ae2b2da",
			"69dfe7c48a763667bad7815f679be9110c8322f3",
			routingReadmeCommit,
			"755b31e21e2722c7acda07de5885454d8cecd1db",
			routingFixtureCommit,
		},
		FreezeCurrent: true,
	},
	"internal/intent/avp_ledger_test.go": {
		SHA256:        "13b640e8c50b5dc2b1c241cf5d301f790a3fca15234696b47c99588d3a75349a",
		History:       []string{routingFixtureCommit},
		FreezeCurrent: true,
	},
	"internal/intent/avp_native_windows_test.go": {
		SHA256:  "620d03d2630ab33ace5f88719f8fab417eee5a7171c84022c2bc33674d208e6e",
		History: []string{routingReadmeCommit, routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/intent/avp_rooted_test.go": {
		SHA256:  "e0a178c3ed90677aefae0c9c25fa4d066dd7377b6edd89a35d6a1272c963e145",
		History: []string{routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/intent/avp_source_scans_test.go": {
		SHA256:  "ce2768f222833e90fc9da07e0cd5e47aa1e4174531ed38e4d3263798f3a54489",
		History: []string{routingReadmeCommit, routingFixtureCommit},
	},
	"internal/intent/avp_status_test.go": {
		SHA256:  "09b5e2be4da6586e0e996d39caae50db7545c93ebf152ef0116dd5de71632fe1",
		History: []string{routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/intent/avp_windows_guards_test.go": {
		SHA256:  "9fc31679ccfa7b02d05f136b3bcd1c50058908a10f518b2e65432c88bde4485c",
		History: []string{routingReadmeCommit, routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/intent/fifo_tripwire_other_test.go": {
		SHA256:  "dbf06b188f3f1b82697ee9e33475fc26cb30357f58fb86a784d37170d27def3f",
		History: []string{routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/intent/fifo_tripwire_unix_test.go": {
		SHA256:  "8dfbdd111c4fe279a4aacbb29efd8b6823b5653e7cdc5b21082f358938c70bbb",
		History: []string{routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/intent/harness_test.go": {
		SHA256:  "c54f9facdd24e26370e740bedb2847e354e170ffd86d224a557fcea2eca28a6e",
		History: []string{routingFixtureCommit}, FreezeCurrent: true,
	},
	"internal/intent/status_schema_test.go": {
		SHA256:  "e6dc84c99c503b70aef0fe84d631925e7073050cd42c7276eaebeb3de5aee113",
		History: []string{routingReadmeCommit, routingFixtureCommit}, FreezeCurrent: true,
	},
}

func TestPreparePIBPreChangeGoldens(t *testing.T) {
	if !rescap.LockSupported {
		t.Skip("runtime baseline uses the resource-capture-supported linux/darwin envelope; PIB-287 has a native unsupported-platform test")
	}
	recording := os.Getenv(preparePIBGoldenRecordEnv) == "1"
	var binary, checkBinary string
	if recording {
		binary = os.Getenv(preparePIBGoldenBinEnv)
		if binary == "" {
			t.Fatalf("%s must name the detached baseline binary in record mode", preparePIBGoldenBinEnv)
		}
		requirePreparePIBBaselineBinary(t, binary)
		checkBinary = os.Getenv(prepareCheckGoldenBinEnv)
		if checkBinary == "" {
			t.Fatalf("%s must name the accepted prepare-check binary in record mode", prepareCheckGoldenBinEnv)
		}
		requirePrepareCheckBaselineBinary(t, checkBinary)
	} else {
		if err := validatePreparePIBComparisonInputs(
			os.Getenv(preparePIBGoldenBinEnv),
			os.Getenv(prepareCheckGoldenBinEnv),
		); err != nil {
			t.Fatal(err)
		}
		binary = buildPreparePIBCurrentBinary(t)
		checkBinary = binary
	}
	captured := capturePreparePIBSurfaces(t, binary, checkBinary)

	if recording {
		recordPreparePIBGoldens(t, captured)
		t.Logf("recorded %d prepare PIB pre-change fixtures", len(captured))
		return
	}

	t.Run("fixture-set-is-closed", func(t *testing.T) {
		assertPreparePIBFixtureAgreement(t, captured)
	})

	for row, selectors := range preparePIBRows {
		row, selectors := row, selectors
		t.Run(row, func(t *testing.T) {
			compared := 0
			for name, got := range captured {
				if !matchesGoldenSelector(name, selectors) {
					continue
				}
				comparePreparePIBGolden(t, name, got)
				compared++
			}
			if compared == 0 {
				t.Fatalf("%s compared no fixture (selectors %q)", row, selectors)
			}
		})
	}

	t.Run("doctor-D1-through-D8", func(t *testing.T) {
		for i := 1; i <= 8; i++ {
			name := fmt.Sprintf("doctor-D%d.txt", i)
			comparePreparePIBGolden(t, name, captured[name])
		}
	})

	t.Run("PIB-209-maps-accepted-cycle", func(t *testing.T) {
		for _, name := range []string{"cycle-skip-execute-transcript.txt", "cycle-final-state.txt"} {
			assertRoutingGoldenDigest(t, name)
		}
	})

	t.Run("PIB-391-routing-provenance-and-immutability", func(t *testing.T) {
		readme, err := os.ReadFile(filepath.Join(routingGoldenDir, "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(readme, []byte(acceptedRoutingBaseline)) {
			t.Fatalf("routing README no longer names accepted WAVE_BASE %s", acceptedRoutingBaseline)
		}
		for name := range routingGoldenSHA256 {
			assertRoutingGoldenDigest(t, name)
			if _, duplicated := captured[name]; duplicated {
				t.Fatalf("prepare PIB suite re-records accepted routing fixture %s", name)
			}
		}
		assertRoutingGoldenHistory(t)
		assertAcceptedCheckSourceHistory(t)
	})

	t.Run("D8-was-selected", func(t *testing.T) {
		got := captured["doctor-D8.txt"]
		for _, want := range []string{
			"$ tpatch --path <workspace> doctor --check D8 --json",
			`"checks_run": 1`,
			"exit 0",
		} {
			if !strings.Contains(got, want) {
				t.Fatalf("D8 fixture does not prove selection; missing %q\n%s", want, got)
			}
		}
		for _, forbidden := range []string{`"check_id": "D1"`, `"check_id": "D7"`} {
			if strings.Contains(got, forbidden) {
				t.Fatalf("D8 fixture ran an unselected check: %q", forbidden)
			}
		}
	})
}

func TestValidateRoutingGoldenHistoryRejectsWrongCommit(t *testing.T) {
	histories := expectedRoutingGoldenHistories()
	histories["changed-apply-help.txt"] = []string{"ffffffffffffffffffffffffffffffffffffffff"}
	accepted := map[string]bool{
		routingFixtureCommit: true,
		routingReadmeCommit:  true,
	}
	if err := validateRoutingGoldenHistories(histories, accepted); err == nil {
		t.Fatal("history validator accepted a routing fixture touched by an unauthorized commit")
	}
}

func TestPreparePIBGoldenProvenance(t *testing.T) {
	t.Run("dispatch-only-baseline-delta", func(t *testing.T) {
		cmd := exec.Command("git", "diff", "--name-only", preparePIBWaveBase+".."+preparePIBBaseline)
		cmd.Dir = avpRepoRoot(t)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("inspect baseline delta: %v", err)
		}
		got := strings.Fields(string(out))
		want := []string{"docs/ROADMAP.md", "docs/handoff/CURRENT.md", "docs/supervisor/LOG.md"}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("%s..%s is not the documented tracking-only delta:\ngot %v\nwant %v",
				preparePIBWaveBase, preparePIBBaseline, got, want)
		}
	})

	t.Run("README-and-manifest", func(t *testing.T) {
		body, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, "README.md"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, want := range []string{
			preparePIBBaseline,
			preparePIBWaveBase,
			preparePIBGoldenBinEnv,
			preparePIBGoldenRecordEnv,
			preparePIBGoldenSHAEnv,
			prepareCheckGoldenBinEnv,
			prepareCheckGoldenSHAEnv,
			"git clone --no-hardlinks",
			"-trimpath",
			"43",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("provenance README does not document %q", want)
			}
		}
		assertPreparePIBManifest(t, text)
		assertPrepareCheckFixtureOrdering(t)
	})

	t.Run("unsupported-platform-source-contract", func(t *testing.T) {
		root := avpRepoRoot(t)
		body, err := os.ReadFile(filepath.Join(root, "internal", "rescap", "lock_unsupported.go"))
		if err != nil {
			t.Fatal(err)
		}
		text := string(body)
		for _, want := range []string{
			"//go:build !linux && !darwin",
			"const LockSupported = false",
			"return nil, Refuse(ReasonResourceLockUnsupported,",
			"resource capture requires a linux or darwin host; this build target has no verified flock(2) primitive",
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("unsupported lock implementation lost %q", want)
			}
		}
		if strings.Contains(text, "//go:build unix") {
			t.Fatal("unsupported lock envelope was broadened to unix")
		}
		cmd := exec.Command("git", "log", "--format=%H", "--", filepath.Join("internal", "rescap", "lock_unsupported.go"))
		cmd.Dir = root
		history, err := cmd.Output()
		if err != nil {
			t.Fatalf("read unsupported lock source history: %v", err)
		}
		if got := strings.Fields(string(history)); strings.Join(got, "\n") != strings.Join(unsupportedLockSourceHistory, "\n") {
			t.Fatalf("unsupported lock source history=%v, want %v", got, unsupportedLockSourceHistory)
		}
		windowsTest := readGoldenFile(t, filepath.Join(root, "internal", "cli", "prepare_pib_golden_windows_test.go"))
		for _, want := range []string{
			"//go:build windows",
			"TestPreparePIBUnsupportedPlatformRuntimeGolden",
			"resource-unsupported-platform.txt",
			"feature\", \"resource\", \"capture",
		} {
			if !strings.Contains(windowsTest, want) {
				t.Fatalf("native Windows golden test lost %q", want)
			}
		}
		workflow := readGoldenFile(t, filepath.Join(root, ".github", "workflows", "ci.yml"))
		for _, want := range []string{
			`name: "Test (Windows GH #23 resource golden — blocking)"`,
			"-run '^TestPreparePIBUnsupportedPlatformRuntimeGolden$'",
			"--- PASS: TestPreparePIBUnsupportedPlatformRuntimeGolden",
		} {
			if !strings.Contains(workflow, want) {
				t.Fatalf("blocking native Windows golden gate lost %q", want)
			}
		}
		want := unsupportedPlatformRuntimeGolden()
		got, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, "resource-unsupported-platform.txt"))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, []byte(want)) {
			t.Fatalf("unsupported-platform refusal fixture drifted\n--- want ---\n%s--- got ---\n%s", want, got)
		}
	})
}

func capturePreparePIBSurfaces(t *testing.T, binary, checkBinary string) map[string]string {
	t.Helper()
	captured := make(map[string]string, preparePIBRuntimeCount)
	capturePrepareCheckGoldens(t, captured, checkBinary)
	captureNextGoldens(t, captured, binary)
	capturePhaseGoldens(t, captured, binary)
	captureCompatibilityGoldens(t, captured, binary)
	captureDoctorGoldens(t, captured, binary)
	captureResourceGoldens(t, captured, binary)
	if len(captured) != preparePIBRuntimeCount {
		t.Fatalf("capture produced %d runtime fixtures, want %d", len(captured), preparePIBRuntimeCount)
	}
	return captured
}

func capturePrepareCheckGoldens(t *testing.T, captured map[string]string, binary string) {
	t.Helper()
	populations := []struct {
		name  string
		slug  string
		setup func(*testing.T, string)
	}{
		{name: "ready", slug: preparePIBSlug, setup: func(t *testing.T, root string) {
			setFeatureState(t, root, preparePIBSlug, "defined")
			writeFeaturePrereqs(t, root, map[string]string{
				"analysis.md":    "# Analysis\n",
				"spec.md":        "# Spec\n",
				"exploration.md": "# Exploration\n",
			})
		}},
		{name: "not-ready", slug: preparePIBSlug, setup: func(*testing.T, string) {}},
		{name: "abort", slug: "missing-feature", setup: func(*testing.T, string) {}},
		{name: "ready-pending", slug: preparePIBSlug, setup: func(t *testing.T, root string) {
			setFeatureState(t, root, preparePIBSlug, "defined")
			writeFeaturePrereqs(t, root, map[string]string{
				"analysis.md":    "# Analysis\n",
				"spec.md":        "# Spec\n",
				"exploration.md": "# Exploration\n",
			})
			seedPreparePIBPendingJournal(t, root)
		}},
	}
	for _, population := range populations {
		population := population
		t.Run("capture-check-"+population.name, func(t *testing.T) {
			root, env := newPreparePIBRepo(t, binary)
			population.setup(t, root)
			before := snapshotWholePrepareCheckState(t, root, env)
			human := runPreparePIBCommand(t, binary, root, env,
				"--path", root, "prepare", population.slug, "--check")
			assertWholePrepareCheckStateUnchanged(t, before, snapshotWholePrepareCheckState(t, root, env),
				"prepare --check "+population.name+" human")
			before = snapshotWholePrepareCheckState(t, root, env)
			jsonOut := runPreparePIBCommand(t, binary, root, env,
				"--path", root, "prepare", population.slug, "--check", "--json")
			assertWholePrepareCheckStateUnchanged(t, before, snapshotWholePrepareCheckState(t, root, env),
				"prepare --check "+population.name+" JSON")
			captured["check-"+population.name+"-human.txt"] = human
			captured["check-"+population.name+"-json.txt"] = jsonOut
		})
	}
	if captured["check-ready-human.txt"] != captured["check-ready-pending-human.txt"] {
		t.Fatal("prepare --check human output changed solely because a pending journal exists")
	}
	if captured["check-ready-json.txt"] != captured["check-ready-pending-json.txt"] {
		t.Fatal("prepare --check JSON output changed solely because a pending journal exists")
	}
}

type preparePIBJournalIdentity struct {
	Exists bool   `json:"exists"`
	SHA256 string `json:"sha256,omitempty"`
	Size   int64  `json:"size,omitempty"`
	Mode   uint32 `json:"mode,omitempty"`
}

type preparePIBJournalEntry struct {
	ArtifactID   string                    `json:"artifact_id"`
	Rel          string                    `json:"rel"`
	Action       string                    `json:"action"`
	Preimage     preparePIBJournalIdentity `json:"preimage"`
	PreimageBlob string                    `json:"preimage_blob,omitempty"`
	NewImage     preparePIBJournalIdentity `json:"new_image"`
	StagedRel    string                    `json:"staged_rel"`
}

type preparePIBJournalFixture struct {
	Version    int                      `json:"version"`
	Slug       string                   `json:"slug"`
	Mode       string                   `json:"mode"`
	RunNonce   string                   `json:"run_nonce"`
	PlanDigest string                   `json:"plan_digest"`
	StageRel   string                   `json:"stage_rel"`
	Entries    []preparePIBJournalEntry `json:"entries"`
}

func seedPreparePIBPendingJournal(t *testing.T, root string) {
	t.Helper()
	const (
		analysisRel = ".tpatch/features/pib-golden/analysis.md"
		stageRel    = ".tpatch/local/intent-prepare/pib-golden/stage-0123456789ab"
		stagedRel   = stageRel + "/analysis.md"
	)
	preimage := []byte("# Analysis\n")
	newImage := []byte("# Analysis\n\nPending replacement.\n")
	preimageSum := sha256.Sum256(preimage)
	newImageSum := sha256.Sum256(newImage)
	preimageHex := hex.EncodeToString(preimageSum[:])
	entry := preparePIBJournalEntry{
		ArtifactID: "analysis",
		Rel:        analysisRel,
		Action:     "replace",
		Preimage: preparePIBJournalIdentity{
			Exists: true, SHA256: preimageHex, Size: int64(len(preimage)), Mode: 0o644,
		},
		PreimageBlob: preimageHex,
		NewImage: preparePIBJournalIdentity{
			Exists: true, SHA256: hex.EncodeToString(newImageSum[:]), Size: int64(len(newImage)), Mode: 0o644,
		},
		StagedRel: stagedRel,
	}
	entries := []preparePIBJournalEntry{entry}
	canonicalEntries, err := json.Marshal(entries)
	if err != nil {
		t.Fatal(err)
	}
	planSum := sha256.Sum256(canonicalEntries)
	journal := preparePIBJournalFixture{
		Version: 1, Slug: preparePIBSlug, Mode: "regenerate",
		RunNonce: "0123456789abcdef", PlanDigest: hex.EncodeToString(planSum[:]),
		StageRel: stageRel, Entries: entries,
	}
	body, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeGoldenFile(t, filepath.Join(root, filepath.FromSlash(stagedRel)), string(newImage))
	writeGoldenFile(t, filepath.Join(root, ".tpatch", "features", preparePIBSlug,
		"artifacts", "intent-archive", "blobs", preimageHex+".blob"), string(preimage))
	journalPath := filepath.Join(root, ".tpatch", "local", "intent-prepare", preparePIBSlug, "journal.json")
	writeGoldenFile(t, journalPath, string(body)+"\n")
	if err := os.Chmod(journalPath, 0o600); err != nil {
		t.Fatal(err)
	}
}

func captureNextGoldens(t *testing.T, captured map[string]string, binary string) {
	t.Helper()
	states := []string{
		"requested", "analyzed", "defined", "implementing", "applied", "active",
		"reconciling", "reconciling-shadow", "blocked", "upstream_merged", "rejected", "unapplied",
	}
	for _, state := range states {
		state := state
		t.Run("capture-next-"+state, func(t *testing.T) {
			root, env := newPreparePIBRepo(t, binary)
			setFeatureState(t, root, preparePIBSlug, state)
			before := snapshotPersistentWriteSet(t, root, preparePIBSlug)
			var out strings.Builder
			out.WriteString(runPreparePIBCommand(t, binary, root, env, "--path", root, "next", preparePIBSlug))
			out.WriteString(runPreparePIBCommand(t, binary, root, env, "--path", root, "next", preparePIBSlug, "--format", "harness-json"))
			assertPersistentWriteSetUnchanged(t, before, snapshotPersistentWriteSet(t, root, preparePIBSlug), "next")
			captured["next-"+strings.ReplaceAll(state, "_", "-")+".txt"] = out.String()
		})
	}
	t.Run("capture-next-defined-exploration-present", func(t *testing.T) {
		root, env := newPreparePIBRepo(t, binary)
		setFeatureState(t, root, preparePIBSlug, "defined")
		writeGoldenFile(t, filepath.Join(root, ".tpatch", "features", preparePIBSlug, "exploration.md"), "pre-authored exploration\n")
		before := snapshotPersistentWriteSet(t, root, preparePIBSlug)
		var out strings.Builder
		out.WriteString(runPreparePIBCommand(t, binary, root, env, "--path", root, "next", preparePIBSlug))
		out.WriteString(runPreparePIBCommand(t, binary, root, env, "--path", root, "next", preparePIBSlug, "--format", "harness-json"))
		assertPersistentWriteSetUnchanged(t, before, snapshotPersistentWriteSet(t, root, preparePIBSlug), "next")
		captured["next-defined-exploration-present.txt"] = out.String()
	})
}

func capturePhaseGoldens(t *testing.T, captured map[string]string, binary string) {
	t.Helper()
	phases := []struct {
		name      string
		state     string
		prereqs   map[string]string
		manual    string
		manualRel string
	}{
		{name: "analyze", state: "requested", manualRel: "analysis.md", manual: "# Manual analysis\n\nFixed analysis.\n"},
		{name: "define", state: "analyzed", prereqs: map[string]string{"analysis.md": "# Analysis\n\nFixed analysis.\n"}, manualRel: "spec.md", manual: "# Manual spec\n\n- Acceptance: fixed.\n"},
		{name: "explore", state: "defined", prereqs: map[string]string{"analysis.md": "# Analysis\n", "spec.md": "# Spec\n"}, manualRel: "exploration.md", manual: "# Manual exploration\n\nUse the existing path.\n"},
		{name: "implement", state: "defined", prereqs: map[string]string{"analysis.md": "# Analysis\n", "spec.md": "# Spec\n", "exploration.md": "# Exploration\n"}, manualRel: filepath.Join("artifacts", "apply-recipe.json"), manual: "{\n  \"feature\": \"pib-golden\",\n  \"operations\": [\n    {\"type\": \"ensure-directory\", \"path\": \"generated\"}\n  ]\n}\n"},
	}
	for _, phase := range phases {
		phase := phase
		t.Run("capture-auto-"+phase.name, func(t *testing.T) {
			root, env := newPreparePIBRepo(t, binary)
			setFeatureState(t, root, preparePIBSlug, phase.state)
			writeFeaturePrereqs(t, root, phase.prereqs)
			before := readStatusInvariant(t, root)
			transcript := runPreparePIBCommand(t, binary, root, env, "--path", root, phase.name, preparePIBSlug)
			assertStatusIdentityAndMutation(t, before, readStatusInvariant(t, root), true, phase.name)
			captured["phase-auto-"+phase.name+".txt"] = transcript + snapshotCommandOwnedWriteSet(t, root, preparePIBSlug)
		})
		t.Run("capture-manual-"+phase.name, func(t *testing.T) {
			root, env := newPreparePIBRepo(t, binary)
			setFeatureState(t, root, preparePIBSlug, phase.state)
			writeFeaturePrereqs(t, root, phase.prereqs)
			writeGoldenFile(t, filepath.Join(root, ".tpatch", "features", preparePIBSlug, phase.manualRel), phase.manual)
			before := readStatusInvariant(t, root)
			transcript := runPreparePIBCommand(t, binary, root, env, "--path", root, phase.name, preparePIBSlug, "--manual")
			assertStatusIdentityAndMutation(t, before, readStatusInvariant(t, root), true, phase.name+" --manual")
			captured["phase-manual-"+phase.name+".txt"] = transcript + snapshotCommandOwnedWriteSet(t, root, preparePIBSlug)
		})
	}
}

func captureCompatibilityGoldens(t *testing.T, captured map[string]string, binary string) {
	t.Helper()
	t.Run("capture-compat-status", func(t *testing.T) {
		root, env := newPreparePIBRepo(t, binary)
		before := snapshotPersistentWriteSet(t, root, preparePIBSlug)
		got := runPreparePIBCommand(t, binary, root, env, "--path", root, "status", preparePIBSlug, "--json")
		assertPersistentWriteSetUnchanged(t, before, snapshotPersistentWriteSet(t, root, preparePIBSlug), "status")
		captured["compat-status.txt"] = got + snapshotCommandOwnedWriteSet(t, root, preparePIBSlug)
	})
	t.Run("capture-compat-verify", func(t *testing.T) {
		root, env := newPreparePIBRepo(t, binary)
		setupRecordedFeature(t, binary, root, env)
		before := snapshotPersistentWriteSet(t, root, preparePIBSlug)
		got := runPreparePIBCommand(t, binary, root, env, "--path", root, "verify", preparePIBSlug, "--json", "--quiet", "--no-write")
		assertPersistentWriteSetUnchanged(t, before, snapshotPersistentWriteSet(t, root, preparePIBSlug), "verify --no-write")
		if !strings.Contains(got, `"checks": [`) || strings.Contains(got, `"checks": []`) {
			t.Fatalf("verify fixture did not execute real checks:\n%s", got)
		}
		captured["compat-verify.txt"] = got + snapshotCommandOwnedWriteSet(t, root, preparePIBSlug)
	})
	t.Run("capture-compat-record", func(t *testing.T) {
		root, env := newPreparePIBRepo(t, binary)
		writeGoldenFile(t, filepath.Join(root, "src", "golden.txt"), "recorded change\n")
		before := readStatusInvariant(t, root)
		got := runPreparePIBCommand(t, binary, root, env, "--path", root, "record", preparePIBSlug)
		assertStatusIdentityAndMutation(t, before, readStatusInvariant(t, root), true, "record")
		captured["compat-record.txt"] = got + snapshotCommandOwnedWriteSet(t, root, preparePIBSlug)
	})
	t.Run("capture-compat-land", func(t *testing.T) {
		root, env := newPreparePIBRepo(t, binary)
		setupRecordedFeature(t, binary, root, env)
		before := snapshotPersistentWriteSet(t, root, preparePIBSlug)
		got := runPreparePIBCommand(t, binary, root, env, "--path", root, "land", preparePIBSlug, "--dry-run")
		assertPersistentWriteSetUnchanged(t, before, snapshotPersistentWriteSet(t, root, preparePIBSlug), "land --dry-run")
		if !strings.Contains(got, "expected patch bytes") || strings.Contains(got, "no current capture") {
			t.Fatalf("land dry-run fixture did not operate on a recorded feature:\n%s", got)
		}
		captured["compat-land.txt"] = got + snapshotCommandOwnedWriteSet(t, root, preparePIBSlug)
	})
	t.Run("capture-compat-reconcile", func(t *testing.T) {
		root, env := newPreparePIBRepo(t, binary)
		setupRecordedFeature(t, binary, root, env)
		head := gitGoldenOutput(t, root, "rev-parse", "HEAD")
		base := gitGoldenOutput(t, root, "rev-parse", "HEAD^")
		writeGoldenFile(t, filepath.Join(root, ".tpatch", "upstream.lock"),
			"remote: \"origin\"\nbranch: \"main\"\ncommit: \""+base+"\"\nurl: \"\"\n")
		before := snapshotPersistentWriteSet(t, root, preparePIBSlug)
		got := runPreparePIBCommand(t, binary, root, env, "--path", root, "reconcile", preparePIBSlug,
			"--upstream-ref", head, "--check-applied-only")
		assertPersistentWriteSetUnchanged(t, before, snapshotPersistentWriteSet(t, root, preparePIBSlug), "reconcile --check-applied-only")
		if strings.Contains(got, "ambiguous argument") ||
			!strings.Contains(got, "Checked pib-golden against "+head) ||
			!strings.Contains(got, "[upstreamed]") ||
			!strings.Contains(got, "matched-upstream-sha: "+head) {
			t.Fatalf("reconcile fixture did not enter feature reconciliation:\n%s", got)
		}
		captured["compat-reconcile.txt"] = got + snapshotCommandOwnedWriteSet(t, root, preparePIBSlug)
	})
}

func captureDoctorGoldens(t *testing.T, captured map[string]string, binary string) {
	t.Helper()
	for i := 1; i <= 8; i++ {
		id := fmt.Sprintf("D%d", i)
		t.Run("capture-doctor-"+id, func(t *testing.T) {
			root, env := newPreparePIBRepo(t, binary)
			setupDoctorGolden(t, root, id)
			args := []string{"--path", root, "doctor", "--check", id, "--json"}
			if id == "D6" {
				args = append(args, "--release-metadata", "release-metadata.json")
			}
			before := snapshotPersistentWriteSet(t, root, preparePIBSlug)
			got := runPreparePIBCommand(t, binary, root, env, args...)
			assertPersistentWriteSetUnchanged(t, before, snapshotPersistentWriteSet(t, root, preparePIBSlug), "doctor "+id)
			if id == "D3" {
				got = normalizeD3BundledDigest(t, got)
			}
			captured["doctor-"+id+".txt"] = got + snapshotCommandOwnedWriteSet(t, root, preparePIBSlug)
		})
	}
}

func setupDoctorGolden(t *testing.T, root, id string) {
	t.Helper()
	featureDir := filepath.Join(root, ".tpatch", "features", preparePIBSlug)
	switch id {
	case "D1":
		path := filepath.Join(featureDir, "status.json")
		body := strings.TrimSpace(readGoldenFile(t, path))
		body = strings.TrimSuffix(body, "}") + ",\n  \"unknown_field\": true\n}\n"
		writeGoldenFile(t, path, body)
		writeGoldenFile(t, filepath.Join(featureDir, "feature.yaml"), "legacy: true\n")
	case "D2":
		writeGoldenFile(t, filepath.Join(featureDir, "artifacts", "post-apply.patch"), "diff --git a/a b/a\n")
	case "D3":
		writeGoldenFile(t, filepath.Join(root, ".github", "skills", "tessera-patch", "SKILL.md"), "# tpatch stale managed skill\n")
	case "D4":
		writeGoldenFile(t, filepath.Join(root, ".tpatch", "upstream.lock"), "remote: origin\nbranch: main\ncommit: abc\n")
	case "D5":
		updateStatusJSON(t, filepath.Join(featureDir, "status.json"), func(status map[string]any) {
			status["state"] = "applied"
			status["reconcile"] = map[string]any{
				"attempted_at": "2026-01-02T03:04:05Z",
				"outcome":      "reapplied",
			}
		})
	case "D6":
		gitGolden(t, root, "tag", "v9.9.9")
		writeGoldenFile(t, filepath.Join(root, "CHANGELOG.md"), "# Changelog\n\n## v9.9.9 - 2026-01-02 - Golden\n\n- Golden.\n")
		writeGoldenFile(t, filepath.Join(root, "release-metadata.json"), "[]\n")
	case "D7":
		writeGoldenFile(t, filepath.Join(featureDir, "artifacts", "apply-recipe.json"), "{\n  \"operations\": [\n")
	case "D8":
		// D8 is intentionally empty; the transcript's checks_run=1 is the proof.
	default:
		t.Fatalf("unknown doctor fixture %s", id)
	}
}

func captureResourceGoldens(t *testing.T, captured map[string]string, binary string) {
	t.Helper()
	scenarios := []struct {
		name string
		run  func(*testing.T, string, string, []string) string
	}{
		{"add", captureResourceAdd},
		{"list", captureResourceList},
		{"remove", captureResourceRemove},
		{"clear", captureResourceClear},
		{"trust-dolt", captureResourceTrustDolt},
		{"capture", captureResourceCapture},
		{"diff", captureResourceDiff},
		{"contention", captureResourceContention},
	}
	for _, scenario := range scenarios {
		scenario := scenario
		t.Run("capture-resource-"+scenario.name, func(t *testing.T) {
			root, env := newResourceGoldenRepo(t, binary)
			beforeStatus := readGoldenFile(t, statusPath(root))
			beforeWrites := snapshotPersistentWriteSet(t, root, preparePIBSlug)
			transcript := scenario.run(t, binary, root, env)
			if gotStatus := readGoldenFile(t, statusPath(root)); gotStatus != beforeStatus {
				t.Fatalf("feature resource %s mutated status.json\n--- before ---\n%s--- after ---\n%s",
					scenario.name, beforeStatus, gotStatus)
			}
			afterWrites := snapshotPersistentWriteSet(t, root, preparePIBSlug)
			expectMutation := true
			if (beforeWrites != afterWrites) != expectMutation {
				t.Fatalf("feature resource %s mutation=%v, want %v", scenario.name, beforeWrites != afterWrites, expectMutation)
			}
			captured["resource-"+scenario.name+".txt"] = transcript + snapshotCommandOwnedWriteSet(t, root, preparePIBSlug)
		})
	}
}

func captureResourceAdd(t *testing.T, binary, root string, env []string) string {
	return runPreparePIBCommand(t, binary, root, env, "--path", root, "feature", "resource", "add", preparePIBSlug,
		"--kind", "ignored-file", "--selector", "config/golden.env", "--json")
}

func captureResourceList(t *testing.T, binary, root string, env []string) string {
	setupResourceAdd(t, binary, root, env, "ignored-file", "config/golden.env")
	return runPreparePIBCommand(t, binary, root, env, "--path", root, "feature", "resource", "list", preparePIBSlug, "--json")
}

func captureResourceRemove(t *testing.T, binary, root string, env []string) string {
	setupResourceAdd(t, binary, root, env, "ignored-file", "config/golden.env")
	got := runPreparePIBCommand(t, binary, root, env, "--path", root, "feature", "resource", "remove", preparePIBSlug,
		ignoredFileResourceID, "--json")
	manifest := readGoldenFile(t, filepath.Join(root, ".tpatch", "features", preparePIBSlug, "artifacts", "resources.json"))
	if strings.Contains(manifest, ignoredFileResourceID) || !strings.Contains(manifest, `"resources": []`) {
		t.Fatalf("resource remove did not remove %s:\n%s", ignoredFileResourceID, manifest)
	}
	return got
}

func captureResourceClear(t *testing.T, binary, root string, env []string) string {
	setupResourceAdd(t, binary, root, env, "ignored-file", "config/golden.env")
	return runPreparePIBCommand(t, binary, root, env, "--path", root, "feature", "resource", "clear", preparePIBSlug, "--json")
}

func captureResourceTrustDolt(t *testing.T, binary, root string, env []string) string {
	setupResourceAdd(t, binary, root, env, "git-metadata", "head")
	got := runPreparePIBCommand(t, binary, root, env, "--path", root, "feature", "resource", "trust-dolt", preparePIBSlug,
		gitMetadataResourceID, "--binary-sha256", strings.Repeat("a", 64), "--json")
	if !strings.Contains(got, rescap.ReasonResourceNotDoltAdapter) || strings.Contains(got, rescap.ReasonNoSuchResource) {
		t.Fatalf("trust-dolt did not select the git-metadata resource:\n%s", got)
	}
	return got
}

func captureResourceCapture(t *testing.T, binary, root string, env []string) string {
	setupResourceAdd(t, binary, root, env, "ignored-file", "config/golden.env")
	setupResourceAdd(t, binary, root, env, "git-metadata", "head")
	return runPreparePIBCommand(t, binary, root, env, "--path", root, "feature", "resource", "capture", preparePIBSlug, "--json")
}

func captureResourceDiff(t *testing.T, binary, root string, env []string) string {
	setupResourceAdd(t, binary, root, env, "ignored-file", "config/golden.env")
	runSetupCommand(t, binary, root, env, "--path", root, "feature", "resource", "capture", preparePIBSlug)
	writeGoldenFile(t, filepath.Join(root, "config", "golden.env"), "A=2\n")
	return runPreparePIBCommand(t, binary, root, env, "--path", root, "feature", "resource", "diff", preparePIBSlug, "--json")
}

func captureResourceContention(t *testing.T, binary, root string, env []string) string {
	setupResourceAdd(t, binary, root, env, "git-metadata", "head")
	lock, err := rescap.AcquireLock(rescap.ScratchRoot(root, preparePIBSlug), root)
	if err != nil {
		t.Fatalf("hold resource lock: %v", err)
	}
	defer func() { _ = lock.Release() }()
	return runPreparePIBCommand(t, binary, root, env, "--path", root, "feature", "resource", "capture", preparePIBSlug)
}

func newPreparePIBRepo(t *testing.T, binary string) (string, []string) {
	t.Helper()
	root := t.TempDir()
	env := hermeticPreparePIBEnv(t)
	gitGolden(t, root, "init", "-q", "-b", "main")
	gitGolden(t, root, "config", "user.name", "Golden User")
	gitGolden(t, root, "config", "user.email", "golden@example.invalid")
	writeGoldenFile(t, filepath.Join(root, "README.md"), "golden baseline\n")
	gitGolden(t, root, "add", "README.md")
	gitGolden(t, root, "commit", "-q", "-m", "golden baseline")
	runSetupCommand(t, binary, root, env, "--path", root, "init")
	runSetupCommand(t, binary, root, env, "--path", root, "add", "--slug", preparePIBSlug, "Golden feature")
	setInitialFeatureTimestamps(t, root)
	removePreparePIBInstalledAssets(t, root)
	gitGolden(t, root, "add", "-A")
	gitGolden(t, root, "commit", "-q", "-m", "initialize tpatch workspace")
	return root, env
}

func newResourceGoldenRepo(t *testing.T, binary string) (string, []string) {
	t.Helper()
	root, env := newPreparePIBRepo(t, binary)
	writeGoldenFile(t, filepath.Join(root, "config", "golden.env"), "A=1\n")
	gitignore := readGoldenFile(t, filepath.Join(root, ".gitignore"))
	writeGoldenFile(t, filepath.Join(root, ".gitignore"), gitignore+"config/golden.env\n")
	gitGolden(t, root, "add", ".gitignore")
	gitGolden(t, root, "commit", "-q", "-m", "ignore golden resource")
	return root, env
}

func setupResourceAdd(t *testing.T, binary, root string, env []string, kind, selector string) {
	t.Helper()
	runSetupCommand(t, binary, root, env, "--path", root, "feature", "resource", "add", preparePIBSlug,
		"--kind", kind, "--selector", selector)
	want := ignoredFileResourceID
	if kind == "git-metadata" {
		want = gitMetadataResourceID
	}
	body := readGoldenFile(t, filepath.Join(root, ".tpatch", "features", preparePIBSlug, "artifacts", "resources.json"))
	if !strings.Contains(body, `"resource_id": "`+want+`"`) {
		t.Fatalf("%s/%s produced an unexpected resource ID:\n%s", kind, selector, body)
	}
}

func hermeticPreparePIBEnv(t *testing.T) []string {
	t.Helper()
	home := t.TempDir()
	env := []string{
		"HOME=" + home,
		"USERPROFILE=" + home,
		"XDG_CONFIG_HOME=" + filepath.Join(home, ".config"),
		"GIT_CONFIG_GLOBAL=" + filepath.Join(home, "gitconfig-absent"),
		"GIT_CONFIG_SYSTEM=" + filepath.Join(home, "gitconfig-absent"),
		"GIT_TERMINAL_PROMPT=0",
		"TPATCH_NO_AUTO_DETECT=1",
		"LC_ALL=C",
		"LANG=C",
		"TZ=UTC",
	}
	for _, name := range []string{"PATH", "SystemRoot", "ComSpec", "TMPDIR", "TEMP", "TMP", "windir"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

func removePreparePIBInstalledAssets(t *testing.T, root string) {
	t.Helper()
	for _, rel := range []string{
		".claude/skills/tessera-patch/SKILL.md",
		".cursor/rules/tessera-patch.mdc",
		".github/prompts/tessera-patch-apply.prompt.md",
		".github/skills/tessera-patch/SKILL.md",
		".tpatch/workflows/tessera-patch-generic.md",
		".windsurfrules",
	} {
		if err := os.Remove(filepath.Join(root, filepath.FromSlash(rel))); err != nil && !os.IsNotExist(err) {
			t.Fatalf("remove init-managed golden asset %s: %v", rel, err)
		}
	}
}

func gitGolden(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = hermeticGitEnv(root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitGoldenOutput(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = root
	cmd.Env = hermeticGitEnv(root)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func hermeticGitEnv(root string) []string {
	env := []string{
		"GIT_AUTHOR_NAME=Golden User",
		"GIT_AUTHOR_EMAIL=golden@example.invalid",
		"GIT_COMMITTER_NAME=Golden User",
		"GIT_COMMITTER_EMAIL=golden@example.invalid",
		"GIT_AUTHOR_DATE=2026-01-02T03:04:05Z",
		"GIT_COMMITTER_DATE=2026-01-02T03:04:05Z",
		"GIT_CONFIG_GLOBAL=" + filepath.Join(root, "gitconfig-absent"),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_COUNT=4",
		"GIT_CONFIG_KEY_0=gc.auto",
		"GIT_CONFIG_VALUE_0=0",
		"GIT_CONFIG_KEY_1=gc.autoDetach",
		"GIT_CONFIG_VALUE_1=false",
		"GIT_CONFIG_KEY_2=maintenance.auto",
		"GIT_CONFIG_VALUE_2=0",
		"GIT_CONFIG_KEY_3=maintenance.autoDetach",
		"GIT_CONFIG_VALUE_3=false",
		"GIT_TERMINAL_PROMPT=0",
		"HOME=" + filepath.Join(root, ".git-home"),
		"LC_ALL=C",
		"LANG=C",
		"TZ=UTC",
	}
	for _, name := range []string{"PATH", "SystemRoot", "ComSpec", "TMPDIR", "TEMP", "TMP", "windir"} {
		if value, ok := os.LookupEnv(name); ok {
			env = append(env, name+"="+value)
		}
	}
	return env
}

func runSetupCommand(t *testing.T, binary, root string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = root
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("setup tpatch %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func runPreparePIBCommand(t *testing.T, binary, root string, env []string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = root
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exit := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exit = exitErr.ExitCode()
		} else {
			t.Fatalf("run tpatch %s: %v", strings.Join(args, " "), err)
		}
	}
	displayArgs := strings.ReplaceAll(strings.Join(args, " "), root, "<workspace>")
	return fmt.Sprintf("$ tpatch %s\nexit %d\nstdout:\n%sstderr:\n%s",
		displayArgs, exit, normalizePreparePIBBytes(stdout.String(), root), normalizePreparePIBBytes(stderr.String(), root))
}

var (
	rfc3339Golden          = `\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?Z`
	namedJSONWallClock     = regexp.MustCompile(`("(?:updated_at|generated_at|verified_at|captured_at|recorded_at)"\s*:\s*")` + rfc3339Golden + `(")`)
	gitVersionPattern      = regexp.MustCompile(`("git_version"\s*:\s*")[^"]+(")`)
	recordedMarkdownClock  = regexp.MustCompile(`(\*\*Recorded\*\*:\s*)` + rfc3339Golden)
	d3BundledDigestPattern = regexp.MustCompile(`bundled sha256=[0-9a-f]{12}`)
)

func normalizePreparePIBBytes(body, root string) string {
	body = strings.ReplaceAll(body, root, "<workspace>")
	body = namedJSONWallClock.ReplaceAllStringFunc(body, func(match string) string {
		if strings.Contains(match, preparePIBOldTime) {
			return match
		}
		return namedJSONWallClock.ReplaceAllString(match, `${1}<wall-clock>${2}`)
	})
	body = gitVersionPattern.ReplaceAllString(body, `${1}<git-version>${2}`)
	return recordedMarkdownClock.ReplaceAllString(body, `${1}<wall-clock>`)
}

func snapshotCommandOwnedWriteSet(t *testing.T, root, slug string) string {
	t.Helper()
	return snapshotWriteSet(t, root, slug, true)
}

func snapshotPersistentWriteSet(t *testing.T, root, slug string) string {
	t.Helper()
	return snapshotWriteSet(t, root, slug, false)
}

func snapshotWriteSet(t *testing.T, root, slug string, normalize bool) string {
	t.Helper()
	targets := []string{
		filepath.Join(".tpatch", "FEATURES.md"),
		filepath.Join(".tpatch", "upstream.lock"),
		filepath.Join(".tpatch", "features", slug),
		filepath.Join(".tpatch", "local", "intent-prepare", slug),
		filepath.Join(".tpatch", "local", "resource-scratch", slug),
	}
	var paths []string
	var absent []string
	for _, rel := range targets {
		base := filepath.Join(root, rel)
		if _, err := os.Lstat(base); os.IsNotExist(err) {
			absent = append(absent, filepath.ToSlash(rel))
			continue
		} else if err != nil {
			t.Fatal(err)
		}
		err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.IsDir() {
				paths = append(paths, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(paths)
	sort.Strings(absent)
	var out strings.Builder
	out.WriteString("command-owned-write-set:\n")
	for _, rel := range absent {
		fmt.Fprintf(&out, "--- %s (absent) ---\n", rel)
	}
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		body := readGoldenFile(t, path)
		if normalize {
			body = normalizePreparePIBBytes(body, root)
		}
		fmt.Fprintf(&out, "--- %s (%d bytes) ---\n%s", filepath.ToSlash(rel), len(body), body)
		if !strings.HasSuffix(body, "\n") {
			out.WriteByte('\n')
		}
	}
	return out.String()
}

func assertPersistentWriteSetUnchanged(t *testing.T, before, after, command string) {
	t.Helper()
	if before != after {
		t.Fatalf("%s mutated its persistent command-owned write set\n--- before ---\n%s--- after ---\n%s",
			command, before, after)
	}
}

func snapshotWholePrepareCheckState(t *testing.T, root string, env []string) string {
	t.Helper()
	home := ""
	for _, item := range env {
		if strings.HasPrefix(item, "HOME=") {
			home = strings.TrimPrefix(item, "HOME=")
			break
		}
	}
	if home == "" {
		t.Fatal("hermetic prepare environment has no HOME")
	}
	return snapshotTreeMetadata(t, "workspace", root) + snapshotTreeMetadata(t, "home", home)
}

func snapshotTreeMetadata(t *testing.T, label, root string) string {
	t.Helper()
	var paths []string
	if err := filepath.WalkDir(root, func(path string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	sort.Strings(paths)
	var out strings.Builder
	fmt.Fprintf(&out, "%s-tree:\n", label)
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		rel = filepath.ToSlash(rel)
		switch {
		case info.IsDir():
			fmt.Fprintf(&out, "D %s %04o\n", rel, info.Mode().Perm())
		case info.Mode()&os.ModeSymlink != 0:
			target, err := os.Readlink(path)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&out, "L %s %04o %s\n", rel, info.Mode().Perm(), target)
		case info.Mode().IsRegular():
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(body)
			fmt.Fprintf(&out, "F %s %04o %d %x\n", rel, info.Mode().Perm(), len(body), sum)
		default:
			fmt.Fprintf(&out, "O %s %s\n", rel, info.Mode())
		}
	}
	return out.String()
}

func assertWholePrepareCheckStateUnchanged(t *testing.T, before, after, command string) {
	t.Helper()
	if before != after {
		t.Fatalf("%s mutated the workspace or hermetic HOME\n--- before ---\n%s--- after ---\n%s",
			command, before, after)
	}
}

type statusInvariant struct {
	ID          string
	Slug        string
	RequestedAt string
	UpdatedAt   string
}

func statusPath(root string) string {
	return filepath.Join(root, ".tpatch", "features", preparePIBSlug, "status.json")
}

func readStatusInvariant(t *testing.T, root string) statusInvariant {
	t.Helper()
	var status store.FeatureStatus
	if err := json.Unmarshal([]byte(readGoldenFile(t, statusPath(root))), &status); err != nil {
		t.Fatal(err)
	}
	if status.ID != preparePIBSlug || status.Slug != preparePIBSlug {
		t.Fatalf("feature identity is not deterministic: id=%q slug=%q", status.ID, status.Slug)
	}
	return statusInvariant{
		ID: status.ID, Slug: status.Slug,
		RequestedAt: status.RequestedAt, UpdatedAt: status.UpdatedAt,
	}
}

func assertStatusIdentityAndMutation(t *testing.T, before, after statusInvariant, updated bool, command string) {
	t.Helper()
	if before.ID != after.ID || before.Slug != after.Slug {
		t.Fatalf("%s changed feature identity: before=%+v after=%+v", command, before, after)
	}
	if before.RequestedAt != after.RequestedAt {
		t.Fatalf("%s changed requested_at: %q -> %q", command, before.RequestedAt, after.RequestedAt)
	}
	if (before.UpdatedAt != after.UpdatedAt) != updated {
		t.Fatalf("%s updated_at changed=%v, want %v (%q -> %q)",
			command, before.UpdatedAt != after.UpdatedAt, updated, before.UpdatedAt, after.UpdatedAt)
	}
}

func setInitialFeatureTimestamps(t *testing.T, root string) {
	t.Helper()
	path := statusPath(root)
	body := readGoldenFile(t, path)
	for _, field := range []string{"requested_at", "updated_at"} {
		re := regexp.MustCompile(`("` + field + `"\s*:\s*")[^"]+(")`)
		if !re.MatchString(body) {
			t.Fatalf("status.json has no %s field", field)
		}
		body = re.ReplaceAllString(body, `${1}`+preparePIBOldTime+`${2}`)
	}
	writeGoldenFile(t, path, body)

	requestPath := filepath.Join(root, ".tpatch", "features", preparePIBSlug, "request.md")
	request := readGoldenFile(t, requestPath)
	created := regexp.MustCompile(`(?m)^(\*\*Created\*\*:\s*).+$`)
	if !created.MatchString(request) {
		t.Fatal("request.md has no Created field")
	}
	writeGoldenFile(t, requestPath, created.ReplaceAllString(request, `${1}`+preparePIBOldTime))

	got := readStatusInvariant(t, root)
	if got.RequestedAt != preparePIBOldTime || got.UpdatedAt != preparePIBOldTime {
		t.Fatalf("failed to initialize fixed status timestamps: %+v", got)
	}
}

func setupRecordedFeature(t *testing.T, binary, root string, env []string) {
	t.Helper()
	featureDir := filepath.Join(root, ".tpatch", "features", preparePIBSlug)
	writeGoldenFile(t, filepath.Join(featureDir, "analysis.md"), "# Analysis\n\nGolden analysis.\n")
	writeGoldenFile(t, filepath.Join(featureDir, "spec.md"), "# Spec\n\nGolden acceptance criteria.\n")
	writeGoldenFile(t, filepath.Join(featureDir, "exploration.md"), "# Exploration\n\nGolden implementation path.\n")
	gitGolden(t, root, "add",
		filepath.ToSlash(filepath.Join(".tpatch", "features", preparePIBSlug, "analysis.md")),
		filepath.ToSlash(filepath.Join(".tpatch", "features", preparePIBSlug, "spec.md")),
		filepath.ToSlash(filepath.Join(".tpatch", "features", preparePIBSlug, "exploration.md")))
	gitGolden(t, root, "commit", "-q", "-m", "add golden intent files")
	setFeatureState(t, root, preparePIBSlug, "applied")
	writeGoldenFile(t, filepath.Join(root, "src", "golden.txt"), "recorded change\n")
	gitGolden(t, root, "add", filepath.ToSlash(filepath.Join("src", "golden.txt")))
	gitGolden(t, root, "commit", "-q", "-m", "apply golden change")
	runSetupCommand(t, binary, root, env, "--path", root, "record", preparePIBSlug, "--from", "HEAD^", "--to", "HEAD")
	status := readStatusInvariant(t, root)
	if status.RequestedAt != preparePIBOldTime {
		t.Fatalf("record setup changed requested_at: %+v", status)
	}
}

func normalizeD3BundledDigest(t *testing.T, body string) string {
	t.Helper()
	if matches := d3BundledDigestPattern.FindAllString(body, -1); len(matches) != 1 {
		t.Fatalf("D3 must expose exactly one real bundled digest, got %d:\n%s", len(matches), body)
	}
	return d3BundledDigestPattern.ReplaceAllString(body, "bundled sha256=<12hex>")
}

func setFeatureState(t *testing.T, root, slug, state string) {
	t.Helper()
	path := filepath.Join(root, ".tpatch", "features", slug, "status.json")
	body := readGoldenFile(t, path)
	re := regexp.MustCompile(`("state"\s*:\s*")[^"]+(")`)
	if !re.MatchString(body) {
		t.Fatalf("status.json has no state field")
	}
	updated := re.ReplaceAllString(body, `${1}`+state+`${2}`)
	writeGoldenFile(t, path, updated)
}

func updateStatusJSON(t *testing.T, path string, mutate func(map[string]any)) {
	t.Helper()
	var status map[string]any
	if err := json.Unmarshal([]byte(readGoldenFile(t, path)), &status); err != nil {
		t.Fatal(err)
	}
	mutate(status)
	body, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeGoldenFile(t, path, string(body)+"\n")
}

func writeFeaturePrereqs(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		writeGoldenFile(t, filepath.Join(root, ".tpatch", "features", preparePIBSlug, rel), body)
	}
}

func writeGoldenFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readGoldenFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func unsupportedPlatformRuntimeGolden() string {
	return "$ tpatch --path <workspace> feature resource capture pib-golden\n" +
		"exit 3\n" +
		"stdout:\n" +
		"stderr:\n" +
		"error: resource-lock-unsupported: resource capture requires a linux or darwin host; this build target has no verified flock(2) primitive\n"
}

func buildPreparePIBCurrentBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "tpatch-current")
	cmd := exec.Command("go", "build", "-trimpath",
		"-ldflags", "-X github.com/tesseracode/tesserapatch/internal/buildinfo.Version="+preparePIBToolVersion,
		"-o", binary, "./cmd/tpatch")
	cmd.Dir = avpRepoRoot(t)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build current golden comparator binary: %v\n%s", err, output)
	}
	return binary
}

func requirePreparePIBBaselineBinary(t *testing.T, binary string) {
	t.Helper()
	requirePreparePIBBinaryRevision(t, binary, preparePIBGoldenSHAEnv, preparePIBBaseline)
}

func requirePrepareCheckBaselineBinary(t *testing.T, binary string) {
	t.Helper()
	requirePreparePIBBinaryRevision(t, binary, prepareCheckGoldenSHAEnv, prepareCheckBaseline)
}

func requirePreparePIBBinaryRevision(t *testing.T, binary, acknowledgementEnv, expected string) {
	t.Helper()
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatalf("read baseline binary build info: %v", err)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if err := validatePreparePIBProducerEvidence(os.Getenv(acknowledgementEnv), expected, settings); err != nil {
		t.Fatalf("%s: %v", binary, err)
	}
}

func validatePreparePIBProducerEvidence(acknowledgement, expected string, settings map[string]string) error {
	if acknowledgement != expected {
		return fmt.Errorf("producer acknowledgement=%q, want %s", acknowledgement, expected)
	}
	revision := settings["vcs.revision"]
	if revision == "" {
		return errors.New("producer has empty vcs.revision; build with VCS metadata enabled")
	}
	if revision != expected {
		return fmt.Errorf("producer vcs.revision=%q, want %s", revision, expected)
	}
	if modified := settings["vcs.modified"]; modified != "false" {
		return fmt.Errorf("producer vcs.modified=%q, want false", modified)
	}
	return nil
}

func validatePreparePIBComparisonInputs(phaseBaseline, checkBaseline string) error {
	if phaseBaseline != "" {
		return fmt.Errorf("%s is record-only; comparison mode always builds current code", preparePIBGoldenBinEnv)
	}
	if checkBaseline != "" {
		return fmt.Errorf("%s is record-only; comparison mode always builds current code", prepareCheckGoldenBinEnv)
	}
	return nil
}

func recordPreparePIBGoldens(t *testing.T, captured map[string]string) {
	t.Helper()
	if os.Getenv(preparePIBGoldenBinEnv) == "" {
		t.Fatalf("%s must name the detached baseline binary in record mode", preparePIBGoldenBinEnv)
	}
	if err := os.MkdirAll(preparePIBGoldenDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range captured {
		if err := os.WriteFile(filepath.Join(preparePIBGoldenDir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func comparePreparePIBGolden(t *testing.T, name, got string) {
	t.Helper()
	if got == "" {
		t.Fatalf("capture did not produce %s", name)
	}
	want, err := os.ReadFile(filepath.Join(preparePIBGoldenDir, name))
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	if !bytes.Equal(want, []byte(got)) {
		t.Fatalf("%s drifted from baseline %s\n--- golden ---\n%s\n--- current ---\n%s",
			name, preparePIBBaseline, want, got)
	}
}

func matchesGoldenSelector(name string, selectors []string) bool {
	for _, selector := range selectors {
		if name == selector || strings.HasSuffix(selector, "-") && strings.HasPrefix(name, selector) {
			return true
		}
	}
	return false
}

func assertPreparePIBFixtureAgreement(t *testing.T, captured map[string]string) {
	t.Helper()
	entries, err := os.ReadDir(preparePIBGoldenDir)
	if err != nil {
		t.Fatal(err)
	}
	recorded := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".txt") {
			continue
		}
		recorded++
		if entry.Name() == "resource-unsupported-platform.txt" {
			body, readErr := os.ReadFile(filepath.Join(preparePIBGoldenDir, entry.Name()))
			if readErr != nil {
				t.Error(readErr)
			} else if string(body) != unsupportedPlatformRuntimeGolden() {
				t.Errorf("unsupported-platform runtime fixture drifted:\n%s", body)
			}
			continue
		}
		got, ok := captured[entry.Name()]
		if !ok {
			t.Errorf("fixture %s is not produced by capture", entry.Name())
			continue
		}
		comparePreparePIBGolden(t, entry.Name(), got)
	}
	for name := range captured {
		if _, err := os.Stat(filepath.Join(preparePIBGoldenDir, name)); err != nil {
			t.Errorf("capture %s has no committed fixture: %v", name, err)
		}
	}
	if recorded != preparePIBGoldenCount || len(captured) != preparePIBRuntimeCount {
		t.Fatalf("recorded=%d captured=%d want recorded=%d captured=%d",
			recorded, len(captured), preparePIBGoldenCount, preparePIBRuntimeCount)
	}
}

func assertPreparePIBManifest(t *testing.T, readme string) {
	t.Helper()
	entries, err := os.ReadDir(preparePIBGoldenDir)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".txt") {
			files[entry.Name()] = true
			if !strings.Contains(readme, "`"+entry.Name()+"`") {
				t.Errorf("fixture %s is absent from README manifest", entry.Name())
			}
		}
	}
	listed := map[string]bool{}
	for _, line := range strings.Split(readme, "\n") {
		if !strings.HasPrefix(line, "| `") || !strings.Contains(line, ".txt`") {
			continue
		}
		name := strings.SplitN(line, "`", 3)[1]
		listed[name] = true
		if !files[name] {
			t.Errorf("README manifest names absent fixture %s", name)
		}
	}
	if len(files) != preparePIBGoldenCount || len(listed) != preparePIBGoldenCount {
		t.Fatalf("directory has %d fixtures and manifest has %d, want %d", len(files), len(listed), preparePIBGoldenCount)
	}
}

func assertPrepareCheckFixtureOrdering(t *testing.T) {
	t.Helper()
	root := avpRepoRoot(t)
	addCommits := map[string]bool{}
	for _, name := range prepareCheckGoldenFixtures {
		rel := filepath.Join("internal", "cli", preparePIBGoldenDir, name)
		cmd := exec.Command("git", "log", "--diff-filter=A", "--format=%H", "--", rel)
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("read first-add history for %s: %v", name, err)
		}
		commits := strings.Fields(string(out))
		if len(commits) == 0 {
			t.Skip("check golden ordering activates once the pre-production golden commit is tracked")
		}
		if len(commits) != 1 {
			t.Fatalf("%s has %d first-add commits, want exactly one: %v", name, len(commits), commits)
		}
		addCommits[commits[0]] = true
	}
	if len(addCommits) != 1 {
		t.Fatalf("check goldens were not introduced by one pre-production commit: %v", addCommits)
	}
	var fixtureCommit string
	for commit := range addCommits {
		fixtureCommit = commit
	}
	cmd := exec.Command("git", "log", "--format=", "--name-only", "-z", "--no-renames",
		preparePIBWaveBase+".."+fixtureCommit)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("inspect pre-golden production ordering: %v", err)
	}
	if err := validatePrepareCheckFixtureOrdering(parsePrepareCheckTouchedPaths(out)); err != nil {
		t.Fatal(err)
	}
}

func parsePrepareCheckTouchedPaths(raw []byte) []string {
	var paths []string
	for _, item := range bytes.Split(raw, []byte{0}) {
		path := strings.TrimSpace(string(item))
		if path != "" {
			paths = append(paths, path)
		}
	}
	return paths
}

func validatePrepareCheckFixtureOrdering(paths []string) error {
	for _, path := range paths {
		slash := filepath.ToSlash(path)
		switch {
		case slash == "go.mod", slash == "go.sum", slash == "SPEC.md", slash == "CHANGELOG.md":
			return fmt.Errorf("mutating production path %s landed before or with check goldens", slash)
		case strings.HasPrefix(slash, "cmd/"):
			return fmt.Errorf("mutating production path %s landed before or with check goldens", slash)
		case strings.HasPrefix(slash, "assets/"):
			return fmt.Errorf("mutating production path %s landed before or with check goldens", slash)
		case strings.HasPrefix(slash, "internal/") &&
			strings.HasSuffix(slash, ".go") &&
			!strings.HasSuffix(slash, "_test.go"):
			return fmt.Errorf("mutating production path %s landed before or with check goldens", slash)
		}
	}
	return nil
}

func assertRoutingGoldenDigest(t *testing.T, name string) {
	t.Helper()
	want, ok := routingGoldenSHA256[name]
	if !ok {
		t.Fatalf("no accepted digest pinned for routing fixture %s", name)
	}
	body, err := os.ReadFile(filepath.Join(routingGoldenDir, name))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	got := hex.EncodeToString(sum[:])
	if got != want {
		t.Fatalf("accepted routing fixture %s changed: sha256=%s want=%s", name, got, want)
	}
}

func expectedRoutingGoldenHistories() map[string][]string {
	histories := make(map[string][]string, len(routingGoldenSHA256))
	for name := range routingGoldenSHA256 {
		histories[name] = []string{routingFixtureCommit}
	}
	histories["README.md"] = []string{routingReadmeCommit, routingFixtureCommit}
	return histories
}

func assertRoutingGoldenHistory(t *testing.T) {
	t.Helper()
	root := avpRepoRoot(t)
	histories := make(map[string][]string, len(routingGoldenSHA256))
	for name := range routingGoldenSHA256 {
		cmd := exec.Command("git", "log", "--format=%H", "--", filepath.Join("internal", "cli", routingGoldenDir, name))
		cmd.Dir = root
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("read routing history for %s: %v", name, err)
		}
		histories[name] = strings.Fields(string(out))
	}

	cmd := exec.Command("git", "rev-list", acceptedRoutingBaseline+".."+acceptedRoutingTip)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("read accepted routing range: %v", err)
	}
	accepted := make(map[string]bool)
	for _, commit := range strings.Fields(string(out)) {
		accepted[commit] = true
	}
	if err := validateRoutingGoldenHistories(histories, accepted); err != nil {
		t.Fatal(err)
	}
}

func assertAcceptedCheckSourceHistory(t *testing.T) {
	t.Helper()
	root := avpRepoRoot(t)
	cmd := exec.Command("git", "log", "--format=", "--name-only",
		acceptedRoutingBaseline+".."+acceptedRoutingTip, "--", "internal/intent", "internal/cli", "assets")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("derive accepted check source set: %v", err)
	}
	derived := map[string]bool{}
	for _, rel := range strings.Fields(string(out)) {
		if !strings.HasSuffix(rel, "_test.go") {
			continue
		}
		exists := exec.Command("git", "cat-file", "-e", acceptedRoutingTip+":"+rel)
		exists.Dir = root
		if exists.Run() == nil {
			derived[filepath.ToSlash(rel)] = true
		}
	}
	if len(derived) != len(acceptedCheckSourceProvenance) {
		t.Fatalf("accepted check source set has %d paths, pinned set has %d: derived=%v",
			len(derived), len(acceptedCheckSourceProvenance), derived)
	}
	for rel, want := range acceptedCheckSourceProvenance {
		if !derived[rel] {
			t.Fatalf("pinned accepted check source %s is absent from the derived closed set", rel)
		}
		show := exec.Command("git", "show", acceptedRoutingTip+":"+rel)
		show.Dir = root
		body, err := show.Output()
		if err != nil {
			t.Fatalf("read accepted check source %s at %s: %v", rel, acceptedRoutingTip, err)
		}
		sum := sha256.Sum256(body)
		if got := hex.EncodeToString(sum[:]); got != want.SHA256 {
			t.Fatalf("accepted check source %s baseline sha256=%s, want %s", rel, got, want.SHA256)
		}
		if want.FreezeCurrent {
			current, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				t.Fatal(err)
			}
			currentSum := sha256.Sum256(current)
			if got := hex.EncodeToString(currentSum[:]); got != want.SHA256 {
				t.Fatalf("frozen accepted check source %s current sha256=%s, want %s", rel, got, want.SHA256)
			}
		}
		history := exec.Command("git", "log", "--format=%H",
			acceptedRoutingBaseline+".."+acceptedRoutingTip, "--", rel)
		history.Dir = root
		historyOut, err := history.Output()
		if err != nil {
			t.Fatalf("read accepted check source history for %s: %v", rel, err)
		}
		if got := strings.Fields(string(historyOut)); strings.Join(got, "\n") != strings.Join(want.History, "\n") {
			t.Fatalf("accepted check source %s history=%v, want %v", rel, got, want.History)
		}
	}
}

func validateRoutingGoldenHistories(histories map[string][]string, accepted map[string]bool) error {
	expected := expectedRoutingGoldenHistories()
	if len(histories) != len(expected) {
		return fmt.Errorf("routing history map has %d paths, want closed set of %d", len(histories), len(expected))
	}
	for name, want := range expected {
		got, ok := histories[name]
		if !ok {
			return fmt.Errorf("routing history map is missing %s", name)
		}
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			return fmt.Errorf("routing fixture %s history=%v, want %v", name, got, want)
		}
		for _, commit := range got {
			if !accepted[commit] {
				return fmt.Errorf("routing fixture %s was touched by %s outside accepted range %s..%s",
					name, commit, acceptedRoutingBaseline, acceptedRoutingTip)
			}
		}
	}
	for name := range histories {
		if _, ok := expected[name]; !ok {
			return fmt.Errorf("routing history map contains unrecognized path %s", name)
		}
	}
	return nil
}
