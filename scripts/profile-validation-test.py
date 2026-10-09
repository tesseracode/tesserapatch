#!/usr/bin/env python3
"""Non-Go controls for the optional macOS baseline profiler."""

import importlib.util
import io
import os
from pathlib import Path
import shlex
import tempfile
import time
import unittest
from unittest import mock


spec = importlib.util.spec_from_file_location(
    "profiler", Path(__file__).with_name("profile-validation.py"))
profiler = importlib.util.module_from_spec(spec)
spec.loader.exec_module(profiler)


def reading(now, free=80, load=5, go=False):
    return {"monotonic": now, "admission_free_percent": free,
            "load1": load, "go_processes": [{"pid": 99}] if go else []}


def ps_row(pid, ppid, rss, name, cpu="0:00.00", started="Thu Oct 8 21:00:21 2026"):
    return "%s %s 0.0 %s %s %s %s\n" % (pid, ppid, rss, cpu, started, name)


class AdmissionTests(unittest.TestCase):
    def test_75_percent_requires_explicit_selection(self):
        default, pilot = profiler.QuietWindow(), profiler.QuietWindow(75)
        for now in range(0, 62, 2):
            self.assertFalse(default.observe(reading(now, free=75)))
            self.assertEqual(pilot.observe(reading(now, free=75)), now == 60)
        self.assertFalse(pilot.observe(reading(62, free=74)))
        with self.assertRaises(ValueError):
            profiler.QuietWindow(70)

    def test_75_percent_retains_load_and_process_checks(self):
        for bad in ({"load": 5.01}, {"go": True}):
            window = profiler.QuietWindow(75)
            for now in range(0, 62, 2):
                self.assertFalse(window.observe(reading(now, free=75, **bad)))

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
            ps_row(10, 1, 20, "/usr/bin/time") +
            ps_row(11, 10, 100, "/tools/go") +
            ps_row(12, 11, 200, "/tmp/cli.test") +
            ps_row(13, 12, 30, "/tools/go") +
            ps_row(90, 1, 900, "/other/go"))
        tree = profiler.descendants(list(reversed(rows)), 10)
        self.assertEqual({p["pid"] for p in tree}, {10, 11, 12, 13})
        self.assertEqual(sum(p["rss_bytes"] for p in tree), 350 * 1024)

    def test_cpu_time_units_and_private_path_omission(self):
        self.assertAlmostEqual(profiler.cpu_seconds("1-01:02:03.04"), 90123.04)
        self.assertAlmostEqual(profiler.cpu_seconds("2:03.04"), 123.04)
        row = profiler.processes(ps_row(
            10, 1, 1, "/private/location/App Helper", cpu="2:03.04"))[0]
        self.assertEqual(row["name"], "App Helper")
        self.assertNotIn("/private", str(row))
        with self.assertRaises(ValueError):
            profiler.cpu_seconds("unknown")
        with self.assertRaises(ValueError):
            profiler.processes("10 1 1.0 20")

    def test_interval_cpu_does_not_inherit_reused_pid_counters(self):
        tracker = profiler.ProcessTracker()
        first = profiler.processes(ps_row(10, 1, 20, "/app", cpu="0:01.00"))
        tracker.annotate(first, 0, first[0]["start_epoch"] - 12)
        self.assertIsNone(first[0]["interval_cpu_percent"])
        self.assertEqual(first[0]["seconds_after_boot_started"], 12)
        second = profiler.processes(ps_row(10, 1, 20, "/app", cpu="0:02.50"))
        tracker.annotate(second, 2, 0)
        self.assertEqual(second[0]["interval_cpu_percent"], 75)
        reused = profiler.processes(ps_row(
            10, 1, 20, "/app", cpu="0:10.00", started="Thu Oct 8 21:10:21 2026"))
        tracker.annotate(reused, 4, 0)
        self.assertIsNone(reused[0]["interval_cpu_percent"])
        backwards = profiler.processes(ps_row(
            10, 1, 20, "/app", cpu="0:09.00", started="Thu Oct 8 21:10:21 2026"))
        tracker.annotate(backwards, 6, 0)
        self.assertIsNone(backwards[0]["interval_cpu_percent"])

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
        pilot = profiler.workload(root, Path("/fixture/go"), "pilot")
        self.assertEqual(pilot, [stages[i] for i in (0, 1, 2, 4, 17)])
        self.assertIn("^TestS7ARRev(11|12|13|14|15).*$", pilot[3][1])
        self.assertIn("^TestS7ObservedAPRegistrationAuthority$", pilot[4][1])
        with self.assertRaises(ValueError):
            profiler.workload(root, Path("/fixture/go"), "unbounded")
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

    def test_expired_budget_prevents_launch_and_stops_an_owned_child(self):
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            log = Path(directory) / "fixture.log"
            with mock.patch.object(profiler, "BUDGET", profiler.Budget(0)):
                with mock.patch.object(profiler.subprocess, "Popen") as launch:
                    with self.assertRaises(profiler.BudgetExceeded):
                        profiler.execute(["/bin/sleep", "30"], self.root, {},
                                         log, io.StringIO(), "fixture")
                    launch.assert_not_called()

            budget = profiler.Budget(60)

            def sample_then_expire(root):
                budget.deadline = -1
                return {"owned_rss_sum_bytes": 0, "owned_processes": [],
                        "go_processes": [], "pressure_level": 1,
                        "admission_free_percent": 80}

            with mock.patch.object(profiler, "BUDGET", budget):
                with mock.patch.object(profiler, "sample", side_effect=sample_then_expire):
                    with mock.patch.object(profiler, "stop_owned", wraps=profiler.stop_owned) as stop:
                        with self.assertRaises(profiler.BudgetExceeded):
                            profiler.execute(["/bin/sleep", "30"], self.root, {},
                                             log, io.StringIO(), "fixture")
                        stop.assert_called_once()
                        self.assertIsNotNone(stop.call_args.args[0].poll())

    def test_75_percent_requires_bounded_pilot_arguments(self):
        base = ["profiler", "--repo", "/fixture", "--toolchain", "/fixture",
                "--out", "/fixture", "--expected-head", "fixture", "--memory-threshold", "75"]
        for args in ([], ["--profile", "pilot"],
                     ["--profile", "pilot", "--max-seconds", "1801"]):
            with mock.patch.object(profiler.sys, "argv", base + args):
                with mock.patch.object(profiler.sys, "stderr", io.StringIO()):
                    with self.assertRaises(SystemExit) as error:
                        profiler.main()
                    self.assertEqual(error.exception.code, 2)

    def test_cleanup_includes_child_in_a_separate_session(self):
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            pid_file = Path(directory) / "child.pid"
            code = (
                "import subprocess,sys,time; from pathlib import Path; "
                "child=subprocess.Popen(['/bin/sleep','30'],start_new_session=True); "
                "Path(sys.argv[1]).write_text(str(child.pid)); time.sleep(30)"
            )
            root = profiler.subprocess.Popen(
                [profiler.sys.executable, "-c", code, str(pid_file)], start_new_session=True)
            try:
                end = time.monotonic() + 5
                while not pid_file.exists() and time.monotonic() < end:
                    time.sleep(0.02)
                self.assertTrue(pid_file.exists())
                child_pid = int(pid_file.read_text())
                profiler.stop_owned(root)
                self.assertIsNotNone(root.poll())
                probe = profiler.subprocess.run(
                    ["/bin/ps", "-p", str(child_pid), "-o", "stat="],
                    capture_output=True, text=True, timeout=3)
                self.assertTrue(not probe.stdout.strip() or probe.stdout.strip().startswith("Z"),
                                probe.stdout)
            finally:
                if root.poll() is None:
                    os.killpg(root.pid, 9)
                    root.wait()


if __name__ == "__main__":
    unittest.main()
