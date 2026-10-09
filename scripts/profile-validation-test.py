#!/usr/bin/env python3
"""Non-Go controls for the optional macOS baseline profiler."""

import importlib.util
import io
from pathlib import Path
import shlex
import tempfile
import unittest
from unittest import mock


spec = importlib.util.spec_from_file_location(
    "profiler", Path(__file__).with_name("profile-validation.py"))
profiler = importlib.util.module_from_spec(spec)
spec.loader.exec_module(profiler)


def reading(now, free=80, load=5, go=False):
    return {"monotonic": now, "admission_free_percent": free,
            "load1": load, "go_processes": [{"pid": 99}] if go else []}


class AdmissionTests(unittest.TestCase):
    def test_full_minute_is_required(self):
        window = profiler.QuietWindow()
        for now in range(0, 60, 2):
            self.assertFalse(window.observe(reading(now)))
        self.assertTrue(window.observe(reading(60)))

    def test_each_bad_condition_resets_the_minute(self):
        for bad in ({"free": 79}, {"load": 5.01}, {"go": True}):
            with self.subTest(bad=bad):
                window = profiler.QuietWindow()
                for now in range(0, 60, 2):
                    self.assertFalse(window.observe(reading(now)))
                self.assertFalse(window.observe(reading(60, **bad)))
                for now in range(62, 122, 2):
                    self.assertFalse(window.observe(reading(now)))
                self.assertTrue(window.observe(reading(122)))

    def test_sampling_gap_and_backward_time_reset(self):
        for restart in (66, 0):
            with self.subTest(restart=restart):
                window = profiler.QuietWindow()
                for now in range(0, 60, 2):
                    window.observe(reading(now))
                self.assertFalse(window.observe(reading(restart)))
                for now in range(restart + 2, restart + 60, 2):
                    self.assertFalse(window.observe(reading(now)))
                self.assertTrue(window.observe(reading(restart + 60)))

    def test_blocked_admission_produces_evidence_and_timeout(self):
        clock = [0]

        def sleep(seconds):
            clock[0] += seconds

        stream = io.StringIO()
        with self.assertRaises(profiler.AdmissionTimeout):
            profiler.admit(stream, "fixture", 60,
                           sampler=lambda: reading(clock[0], free=71),
                           clock=lambda: clock[0], sleep=sleep)
        self.assertIn('"phase": "admission"', stream.getvalue())
        self.assertIn('"admission_free_percent": 71', stream.getvalue())


class TelemetryTests(unittest.TestCase):
    def setUp(self):
        self.root = Path(__file__).resolve().parents[1]
        (self.root / "bin").mkdir(exist_ok=True)

    def test_nested_children_count_but_unrelated_go_is_not_owned(self):
        rows = profiler.processes(
            "10 1 0.0 20 /usr/bin/time\n"
            "11 10 80.0 100 /tools/go\n"
            "12 11 10.0 200 /tmp/cli.test\n"
            "13 12 5.0 30 /tools/go\n"
            "90 1 70.0 900 /other/go\n")
        tree = profiler.descendants(list(reversed(rows)), 10)
        self.assertEqual({p["pid"] for p in tree}, {10, 11, 12, 13})
        self.assertEqual(sum(p["rss_bytes"] for p in tree), 350 * 1024)

    def test_missing_telemetry_is_error_not_zero(self):
        with self.assertRaises(ValueError):
            profiler.match(r"free percentage:\s+(\d+)%", "unavailable")

    def test_shards_are_exact_source_commands_and_mutations_refuse(self):
        root = self.root
        stages = profiler.workload(root, Path("/fixture/go"))
        self.assertEqual(len(stages), 25)
        original = (root / "scripts/wave-close-test-shards.sh").read_text()
        expected = [shlex.split(line)[1:] for line in original.splitlines()
                    if line.startswith("go test ")]
        self.assertEqual([cmd[1:] for _, cmd in stages[3:]], expected)
        for changed in (
            original.replace("GOMAXPROCS=1", "GOMAXPROCS=2", 1),
            original.replace("go test -p=1 ", "go test ", 1),
            original.replace("-count=1", "-count=2", 1),
            original.replace("-timeout 40m", "-timeout 60m", 1),
            original[:original.rfind("go test ")],
        ):
            self.assertNotEqual(changed, original)
            with tempfile.TemporaryDirectory(dir=root / "bin") as directory:
                fixture = Path(directory)
                (fixture / "scripts").mkdir()
                (fixture / "scripts/wave-close-test-shards.sh").write_text(changed)
                with self.assertRaises(ValueError):
                    profiler.workload(fixture, Path("/fixture/go"))

    def test_critical_pressure_stops_only_new_owned_group(self):
        value = {
            "owned_rss_sum_bytes": 0, "owned_processes": [], "go_processes": [],
            "pressure_level": 4, "admission_free_percent": 10,
        }
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            with mock.patch.object(profiler, "sample", return_value=value):
                with mock.patch.object(profiler, "stop_owned", wraps=profiler.stop_owned) as stop:
                    with self.assertRaisesRegex(RuntimeError, "critical"):
                        profiler.execute(
                            ["/bin/sleep", "30"], self.root, {},
                            Path(directory) / "fixture.log", io.StringIO(), "fixture")
                    stop.assert_called_once()
                    self.assertIsNotNone(stop.call_args.args[0].poll())


if __name__ == "__main__":
    unittest.main()
