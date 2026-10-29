#!/usr/bin/env python3
"""Non-Go controls for the optional macOS baseline profiler."""

import importlib.util
import io
import json
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


def reading(now, free=80, load=5, go=False, pressure=1, swapouts=0):
    return {"monotonic": now, "admission_free_percent": free,
            "load1": load, "go_processes": [{"pid": 99}] if go else [],
            "pressure_level": pressure, "vm_pages": {"Swapouts": swapouts},
            "swap_used_bytes": 0}


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


class PressurePolicyTests(unittest.TestCase):
    def test_swap_tolerance_is_bounded_relative_to_the_fixed_start_not_previous_sample(self):
        cap = 2 * 1024**3
        start = 267 * 1024**2
        previous = {**reading(0, swapouts=5), "swap_used_bytes": start}
        boundary = {**reading(2, swapouts=500), "swap_used_bytes": start + cap}
        profiler.require_runtime_pressure_safe(boundary, previous, cap, start)
        with self.assertRaises(profiler.ResourceStop):
            profiler.require_runtime_pressure_safe(boundary, previous)
        exceeded = {**reading(4, swapouts=501), "swap_used_bytes": start + cap + 1}
        with self.assertRaisesRegex(profiler.ResourceStop, "beyond"):
            profiler.require_runtime_pressure_safe(exceeded, boundary, cap, start)
        reduced = {**reading(4, swapouts=501), "swap_used_bytes": start - 1}
        profiler.require_runtime_pressure_safe(reduced, boundary, cap, start)

    def test_swap_tolerance_cannot_bypass_pressure_monitoring_or_counter_checks(self):
        start, cap = 1024, 2 * 1024**3
        for value in (reading(2, pressure=2), reading(2, pressure=4), reading(6), reading(-1)):
            with self.assertRaises(profiler.ResourceStop):
                profiler.require_runtime_pressure_safe(value, reading(0), cap, start)
        with self.assertRaises(ValueError):
            profiler.require_runtime_pressure_safe(reading(2), reading(0, swapouts=1), cap, start)
        for usage in (None, -1, "100", True):
            with self.assertRaises(ValueError):
                profiler.require_runtime_pressure_safe(
                    {**reading(2), "swap_used_bytes": usage}, reading(0), cap, start)
        with self.assertRaises(ValueError):
            profiler.require_runtime_pressure_safe(reading(2), reading(0), cap, None)

    def test_external_go_permission_is_explicit_and_preserves_the_quiet_minute(self):
        strict, allowed = profiler.PressureWindow(), profiler.PressureWindow(allow_external_go=True)
        for now in range(0, 62, 2):
            value = reading(now, free=50, go=True)
            self.assertFalse(strict.observe(value))
            self.assertEqual(allowed.observe(value), now == 60)

    def test_external_go_permission_keeps_pressure_load_and_swap_checks(self):
        for condition in ({"pressure": 2}, {"pressure": 4}, {"load": 5.01}):
            window = profiler.PressureWindow(allow_external_go=True)
            for now in range(0, 62, 2):
                self.assertFalse(window.observe(reading(now, go=True, **condition)))
        window = profiler.PressureWindow(allow_external_go=True)
        for now in range(0, 60, 2):
            self.assertFalse(window.observe(reading(now, go=True)))
        self.assertFalse(window.observe(reading(60, go=True, swapouts=1)))
        for now in range(62, 122, 2):
            self.assertFalse(window.observe(reading(now, go=True, swapouts=1)))
        self.assertTrue(window.observe(reading(122, go=True, swapouts=1)))

    def test_pressure_mode_ignores_percentage_but_preserves_the_full_minute(self):
        window = profiler.PressureWindow()
        for now in range(0, 62, 2):
            value = reading(now, free=20, swapouts=100)
            value["swap_used_bytes"] = 1024**3
            self.assertEqual(window.observe(value), now == 60)
        self.assertEqual(window.latest["vm_pages"]["Swapouts"], 100)
        legacy = profiler.QuietWindow()
        for now in range(0, 62, 2):
            self.assertFalse(legacy.observe(reading(now, free=20)))

    def test_warning_critical_load_and_go_processes_prevent_admission(self):
        for condition in ({"pressure": 2}, {"pressure": 4}, {"load": 5.01}, {"go": True}):
            with self.subTest(condition=condition):
                window = profiler.PressureWindow()
                for now in range(0, 62, 2):
                    self.assertFalse(window.observe(reading(now, free=99, **condition)))

    def test_swapout_increment_requires_an_entire_new_quiet_minute(self):
        window = profiler.PressureWindow()
        for now in range(0, 60, 2):
            self.assertFalse(window.observe(reading(now, free=60, swapouts=30)))
        self.assertFalse(window.observe(reading(60, free=60, swapouts=31)))
        for now in range(62, 122, 2):
            self.assertFalse(window.observe(reading(now, free=60, swapouts=31)))
        self.assertTrue(window.observe(reading(122, free=60, swapouts=31)))

    def test_sampling_gaps_and_backward_time_reset_pressure_admission(self):
        for restart in (66, 0):
            window = profiler.PressureWindow()
            for now in range(0, 60, 2):
                window.observe(reading(now, free=50))
            for now in range(restart, restart + 60, 2):
                self.assertFalse(window.observe(reading(now, free=50)))
            self.assertTrue(window.observe(reading(restart + 60, free=50)))

    def test_pressure_telemetry_cannot_be_missing_unknown_or_reset(self):
        for value in (
            {}, {"pressure_level": 1, "vm_pages": {}},
            reading(0, pressure=0), reading(0, pressure=True),
            reading(0, swapouts=-1), reading(0, swapouts="0"),
            reading(float("nan")), reading(0, load=float("inf")),
        ):
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    profiler.PressureWindow().observe(value)
        window = profiler.PressureWindow()
        window.observe(reading(0, swapouts=5))
        with self.assertRaisesRegex(ValueError, "backwards"):
            window.observe(reading(2, swapouts=4))

    def test_runtime_guard_rejects_pressure_swapouts_and_monitoring_gaps(self):
        previous = reading(0, free=50, swapouts=5)
        profiler.require_runtime_pressure_safe(reading(2, free=10, swapouts=5), previous)
        for current in (
            reading(2, pressure=2, swapouts=5),
            reading(2, pressure=4, swapouts=5),
            reading(2, swapouts=6), reading(6, swapouts=5),
            reading(-1, swapouts=5),
        ):
            with self.subTest(current=current):
                with self.assertRaises(profiler.ResourceStop):
                    profiler.require_runtime_pressure_safe(current, previous)
        with self.assertRaises(ValueError):
            profiler.require_runtime_pressure_safe(reading(2, swapouts=4), previous)

    def test_admit_uses_the_explicit_pressure_window(self):
        clock = [0]

        def sleep(seconds):
            clock[0] += seconds

        stream = io.StringIO()
        window = profiler.PressureWindow()
        elapsed = profiler.admit(
            stream, "fixture", 60, sampler=lambda: reading(clock[0], free=40, swapouts=5),
            clock=lambda: clock[0], sleep=sleep, window=window)
        self.assertEqual(elapsed, 60)
        self.assertEqual(window.latest["monotonic"], 60)


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
        self.assertEqual(len(stages), 42)
        original = (root / "scripts/wave-close-test-shards.sh").read_text()
        expected = [shlex.split(line)[1:] for line in original.splitlines()
                    if line.startswith("go test ")]
        self.assertEqual([cmd[1:] for _, cmd in stages[3:]], expected)
        self.assertEqual(profiler.workload(root, Path("/fixture/go"), "partition"), stages[3:])
        self.assertEqual(profiler.workload(root, Path("/fixture/go"), "broad"), stages[3:19])
        pilot = profiler.workload(root, Path("/fixture/go"), "pilot")
        self.assertEqual(pilot, [stages[i] for i in (0, 1, 2, 19, 34)])
        self.assertIn("^TestS7ARRev(11|12|13|14|15).*$", pilot[3][1])
        self.assertIn("^TestS7ObservedAPRegistrationAuthority$", pilot[4][1])
        self.assertEqual(profiler.workload(root, Path("/fixture/go"), "ap-observer"),
                         [stages[34]])
        with self.assertRaises(ValueError):
            profiler.workload(root, Path("/fixture/go"), "unbounded")
        for changed in (
            original.replace("GOMAXPROCS=1", "GOMAXPROCS=2", 1),
            original.replace("go test -p=1 ", "go test ", 1),
            original.replace("-count=1", "-count=2", 1),
            original.replace("-timeout 40m", "-timeout 60m", 1),
            original[:original.rfind("go test ")],
            original.replace("-run '^TestS6[A-F].*$'", "-run '^TestAVP.*$'", 1),
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

    def test_pressure_policy_rejects_unscoped_or_mixed_percentage_modes(self):
        base = ["profiler", "--repo", "/fixture", "--toolchain", "/fixture",
                "--out", "/fixture", "--expected-head", "fixture", "--admission-policy", "pressure"]
        for args in ([], ["--profile", "pilot", "--max-seconds", "1800"],
                     ["--profile", "partition", "--memory-threshold", "75"]):
            with mock.patch.object(profiler.sys, "argv", base + args):
                with mock.patch.object(profiler.sys, "stderr", io.StringIO()):
                    with self.assertRaises(SystemExit) as error:
                        profiler.main()
                    self.assertEqual(error.exception.code, 2)

    def test_swap_tolerance_requires_only_broad_pressure_probe_with_one_hour_cap(self):
        base = ["profiler", "--repo", "/fixture", "--toolchain", "/fixture",
                "--out", "/fixture", "--expected-head", "fixture", "--max-swap-growth-mib", "2048"]
        for args in (
            [],
            ["--profile", "partition", "--admission-policy", "pressure", "--max-seconds", "3600"],
            ["--profile", "broad", "--max-seconds", "3600"],
            ["--profile", "broad", "--admission-policy", "pressure"],
            ["--profile", "broad", "--admission-policy", "pressure", "--max-seconds", "3601"],
        ):
            with mock.patch.object(profiler.sys, "argv", base + args):
                with mock.patch.object(profiler.sys, "stderr", io.StringIO()):
                    with self.assertRaises(SystemExit) as error:
                        profiler.main()
                    self.assertEqual(error.exception.code, 2)

    def test_bounded_swap_execution_and_final_sample_enforce_growth_limit(self):
        start, cap = 500, 2 * 1024**3
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            def value(now, used, count=0):
                return {**reading(now, swapouts=count), "swap_used_bytes": used,
                        "owned_processes": [], "owned_rss_sum_bytes": 0}
            samples = [value(1, start), value(2, start + cap, 1), value(3, start + cap, 2)]
            with mock.patch.object(profiler, "sample", side_effect=samples):
                with mock.patch.object(profiler.BUDGET, "pause", side_effect=lambda _: time.sleep(0.1)):
                    result = profiler.execute(["/bin/sleep", "0.05"], self.root, {},
                                              Path(directory) / "pass.log", io.StringIO(), "fixture",
                                              pressure_baseline=value(0, start),
                                              max_swap_growth_bytes=cap)
            self.assertEqual(result["exit_code"], 0)
            self.assertEqual(result["swap_used_at_start_bytes"], start)
            self.assertEqual(result["sampled_peak_swap_growth_bytes"], cap)
            samples[-1] = value(3, start + cap + 1, 2)
            with mock.patch.object(profiler, "sample", side_effect=samples):
                with mock.patch.object(profiler.BUDGET, "pause", side_effect=lambda _: time.sleep(0.1)):
                    with self.assertRaises(profiler.ResourceStop):
                        profiler.execute(["/bin/sleep", "0.05"], self.root, {},
                                         Path(directory) / "stop.log", io.StringIO(), "fixture",
                                         pressure_baseline=value(0, start),
                                         max_swap_growth_bytes=cap)
            with mock.patch.object(profiler, "sample", return_value=value(1, start, 1)):
                with mock.patch.object(profiler.subprocess, "Popen") as launch:
                    with self.assertRaises(profiler.ResourceStop):
                        profiler.execute(["/bin/sleep", "30"], self.root, {},
                                         Path(directory) / "never.log", io.StringIO(), "fixture",
                                         pressure_baseline=value(0, start),
                                         max_swap_growth_bytes=cap)
                    launch.assert_not_called()

    def test_running_swap_growth_abort_stops_only_owned_work(self):
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            cap = 2 * 1024**3
            over = {**reading(2, swapouts=1), "swap_used_bytes": cap + 1,
                    "owned_processes": [], "owned_rss_sum_bytes": 0}
            other = profiler.subprocess.Popen(["/bin/sleep", "30"], start_new_session=True)
            try:
                with mock.patch.object(profiler, "sample", side_effect=[reading(1), over]):
                    with mock.patch.object(profiler, "stop_owned", wraps=profiler.stop_owned) as stop:
                        with self.assertRaisesRegex(profiler.ResourceStop, "beyond"):
                            profiler.execute(["/bin/sleep", "30"], self.root, {},
                                             Path(directory) / "stop.log", io.StringIO(), "fixture",
                                             pressure_baseline=reading(0), max_swap_growth_bytes=cap)
                        stop.assert_called_once()
                        self.assertIsNotNone(stop.call_args.args[0].poll())
                        self.assertIsNone(other.poll())
            finally:
                profiler.stop_owned(other)

    def test_external_go_flag_cannot_relax_legacy_percentage_policy(self):
        args = ["profiler", "--repo", "/fixture", "--toolchain", "/fixture",
                "--out", "/fixture", "--expected-head", "fixture",
                "--profile", "partition", "--allow-external-go"]
        with mock.patch.object(profiler.sys, "argv", args):
            with mock.patch.object(profiler.sys, "stderr", io.StringIO()):
                with self.assertRaises(SystemExit) as error:
                    profiler.main()
                self.assertEqual(error.exception.code, 2)

    def test_external_go_permission_applies_to_prelaunch_without_changing_command(self):
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            root = Path(directory)
            running = {
                **reading(4, go=True), "owned_processes": [], "owned_rss_sum_bytes": 0,
                "swap_used_bytes": 0,
            }
            samples = [
                {**reading(2, go=True), "swap_used_bytes": 0},
                running,
                {**reading(6, go=True), "swap_used_bytes": 0},
            ]
            with mock.patch.object(profiler, "sample", side_effect=samples):
                with mock.patch.object(profiler.BUDGET, "pause", side_effect=lambda _: time.sleep(0.1)):
                    result = profiler.execute(["/bin/sleep", "0.05"], self.root, {},
                                              root / "fixture.log", io.StringIO(), "fixture",
                                              pressure_baseline=reading(0, go=True),
                                              allow_external_go=True)
            self.assertEqual(result["exit_code"], 0)
            self.assertEqual(result["command"], ["/bin/sleep", "0.05"])
            self.assertEqual(result["sampled_peak_external_go_process_count"], 1)
            for value in (reading(2, go=True, pressure=2), reading(2, go=True, swapouts=1),
                          reading(2, go=True, load=5.01)):
                with mock.patch.object(profiler, "sample", return_value=value):
                    with mock.patch.object(profiler.subprocess, "Popen") as launch:
                        with self.assertRaises(profiler.ResourceStop):
                            profiler.execute(["/bin/sleep", "30"], self.root, {},
                                             root / "never.log", io.StringIO(), "fixture",
                                             pressure_baseline=reading(0, go=True),
                                             allow_external_go=True)
                        launch.assert_not_called()

    def test_external_go_permission_does_not_bypass_runtime_safety(self):
        for condition in ({"pressure": 2}, {"pressure": 4}, {"swapouts": 1}):
            with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
                unhealthy = {
                    **reading(4, go=True, **condition), "owned_processes": [],
                    "owned_rss_sum_bytes": 0, "swap_used_bytes": 0,
                }
                other = profiler.subprocess.Popen(["/bin/sleep", "30"], start_new_session=True)
                try:
                    with mock.patch.object(profiler, "sample",
                                           side_effect=[reading(2, go=True), unhealthy]):
                        with mock.patch.object(profiler, "stop_owned", wraps=profiler.stop_owned) as stop:
                            with self.assertRaises(profiler.ResourceStop):
                                profiler.execute(["/bin/sleep", "30"], self.root, {},
                                                 Path(directory) / "fixture.log", io.StringIO(),
                                                 "fixture", pressure_baseline=reading(0, go=True),
                                                 allow_external_go=True)
                            stop.assert_called_once()
                            self.assertIsNotNone(stop.call_args.args[0].poll())
                            self.assertIsNone(other.poll())
                finally:
                    profiler.stop_owned(other)

    def test_pressure_prelaunch_blocks_before_any_workload_is_started(self):
        for current in (reading(2, pressure=2), reading(2, swapouts=1),
                        reading(2, load=5.1), reading(2, go=True)):
            with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
                with mock.patch.object(profiler, "sample", return_value=current):
                    with mock.patch.object(profiler.subprocess, "Popen") as launch:
                        with self.assertRaises(profiler.ResourceStop):
                            profiler.execute(["/bin/sleep", "30"], self.root, {},
                                             Path(directory) / "never.log", io.StringIO(),
                                             "fixture", pressure_baseline=reading(0))
                        launch.assert_not_called()

    def test_pressure_runtime_stop_reaps_only_owned_workload(self):
        for condition in ({"pressure": 2}, {"pressure": 4}, {"swapouts": 1}):
            with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
                unhealthy = {
                    **reading(4, **condition), "owned_processes": [],
                    "owned_rss_sum_bytes": 0, "swap_used_bytes": 0,
                }
                other = profiler.subprocess.Popen(["/bin/sleep", "30"], start_new_session=True)
                try:
                    with mock.patch.object(profiler, "sample", side_effect=[reading(2), unhealthy]):
                        with mock.patch.object(profiler, "stop_owned", wraps=profiler.stop_owned) as stop:
                            with self.assertRaises(profiler.ResourceStop):
                                profiler.execute(["/bin/sleep", "30"], self.root, {},
                                                 Path(directory) / "fixture.log", io.StringIO(),
                                                 "fixture", pressure_baseline=reading(0))
                            stop.assert_called_once()
                            self.assertIsNotNone(stop.call_args.args[0].poll())
                            self.assertIsNone(other.poll())
                finally:
                    profiler.stop_owned(other)

    def test_completion_sample_cannot_hide_new_swapouts(self):
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            running = {
                **reading(2), "owned_processes": [], "owned_rss_sum_bytes": 0,
                "swap_used_bytes": 0,
            }
            stream = io.StringIO()
            with mock.patch.object(profiler, "sample",
                                   side_effect=[reading(1), running, reading(3, swapouts=1)]):
                with mock.patch.object(profiler.BUDGET, "pause", side_effect=lambda _: time.sleep(0.1)):
                    with self.assertRaises(profiler.ResourceStop):
                        profiler.execute(["/bin/sleep", "0.05"], self.root, {},
                                         Path(directory) / "fixture.log", stream, "fixture",
                                         pressure_baseline=reading(0))
            self.assertIn('"phase": "completion"', stream.getvalue())

    def test_cache_copy_rejects_mismatch_and_preserves_original(self):
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            root = Path(directory)
            source, destination = root / "source", root / "destination"
            (source / "cache/build").mkdir(parents=True)
            (source / "cache/gopath").mkdir()
            (source / "cache/build/fixture").write_text("original")
            record = {"head": "expected", "toolchain_version_file": "version", "status": "validation-failed"}
            (source / "summary.json").write_text(json.dumps(record))
            for head, version in (("wrong", "version"), ("expected", "wrong")):
                with self.assertRaises(ValueError):
                    profiler.seed_cache(source, destination, head, version)
                self.assertFalse(destination.exists())
            result = profiler.seed_cache(source, destination, "expected", "version")
            self.assertEqual(result["source_run"], "source")
            (destination / "cache/build/fixture").write_text("modified-copy")
            self.assertEqual((source / "cache/build/fixture").read_text(), "original")
            record["status"] = "running"
            (source / "summary.json").write_text(json.dumps(record))
            with self.assertRaises(ValueError):
                profiler.seed_cache(source, root / "refused", "expected", "version")

    def test_child_output_tee_preserves_bytes_arguments_stderr_and_failure(self):
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            root = Path(directory)
            toolchain = root / "toolchain"
            (toolchain / "bin").mkdir(parents=True)
            fake = toolchain / "bin/go"
            payload = b'{"Action":"output","Output":"fixture"}\n' * 40000 + b"\x00\xff"
            # A file fixture avoids command-line length limits for the large byte stream.
            data = root / "bytes"
            data.write_bytes(payload)
            fake.write_text(
                "#!" + profiler.sys.executable + "\n"
                "from pathlib import Path\nimport sys\n"
                "assert sys.argv[1:] == ['test','-json','-overlay=fixture','-timeout=8m']\n"
                "sys.stdout.buffer.write(Path(" + repr(str(data)) + ").read_bytes())\n"
                "sys.stderr.write('fixture-stderr\\n')\nsys.exit(7)\n")
            fake.chmod(0o700)
            env = dict(os.environ)
            profiler.capture_child_output(root, toolchain, env)
            wrapper = root / "wrappers/go"
            args = [str(wrapper), "test", "-json", "-overlay=fixture", "-timeout=8m"]
            result = profiler.subprocess.run(args, env=env, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 7)
            self.assertEqual(result.stdout, payload)
            self.assertEqual(result.stderr, b"fixture-stderr\n")
            logs = list((root / "child-output").glob("*.jsonl"))
            self.assertEqual(len(logs), 1)
            self.assertEqual(logs[0].read_bytes(), payload)
            self.assertEqual(logs[0].stat().st_mode & 0o777, 0o600)
            env["TPATCH_PROFILE_CHILD_OUTPUT_DIR"] = str(root / "absent")
            result = profiler.subprocess.run(args, env=env, capture_output=True, timeout=10)
            self.assertNotEqual(result.returncode, 0)
            self.assertEqual(result.stdout, b"")

    def test_nonobserver_go_is_exec_passthrough_without_capture(self):
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            root = Path(directory)
            toolchain = root / "toolchain"
            (toolchain / "bin").mkdir(parents=True)
            fake = toolchain / "bin/go"
            fake.write_text("#!/bin/sh\nprintf '%s\\n' \"$@\"\nexit 9\n")
            fake.chmod(0o700)
            env = dict(os.environ)
            profiler.capture_child_output(root, toolchain, env)
            result = profiler.subprocess.run([str(root / "wrappers/go"), "build", "./..."],
                                             env=env, capture_output=True, timeout=10)
            self.assertEqual(result.returncode, 9)
            self.assertEqual(result.stdout, b"build\n./...\n")
            self.assertEqual(list((root / "child-output").iterdir()), [])

    def test_exec_hook_restores_capture_after_go_test_reorders_path(self):
        with tempfile.TemporaryDirectory(dir=self.root / "bin") as directory:
            root = Path(directory)
            toolchain = root / "toolchain"
            (toolchain / "bin").mkdir(parents=True)
            fake = toolchain / "bin/go"
            event = {
                "Action": "run", "Package": "github.com/tesseracode/tesserapatch/internal/cli",
                "Test": "TestS7APDryRunContracts",
            }
            payload = json.dumps(event) + "\n"
            fake.write_text("#!" + profiler.sys.executable + "\nimport sys\n"
                            "sys.stdout.write(" + repr(payload) + ")\nsys.exit(7)\n")
            fake.chmod(0o700)
            binary = root / "test-binary"
            binary.write_text(
                "#!" + profiler.sys.executable + "\nimport subprocess,sys\n"
                "assert sys.argv[1:] == ['-test.run=unchanged','-test.timeout=8m']\n"
                "sys.exit(subprocess.call(['go','test','-json','-overlay=fixture']))\n")
            binary.chmod(0o700)
            env = dict(os.environ)
            profiler.capture_child_output(root, toolchain, env)
            env["PATH"] = str(toolchain / "bin") + os.pathsep + env["PATH"]
            argv = [str(binary), "-test.run=unchanged", "-test.timeout=8m"]
            bypassed = profiler.subprocess.run(argv, env=env, capture_output=True, timeout=10)
            self.assertEqual(bypassed.returncode, 7)
            self.assertEqual(bypassed.stdout, payload.encode())
            with self.assertRaisesRegex(RuntimeError, "not captured"):
                profiler.verify_child_output(root)
            captured = profiler.subprocess.run([str(root / "wrappers/test-exec")] + argv,
                                               env=env, capture_output=True, timeout=10)
            self.assertEqual(captured.returncode, 7)
            self.assertEqual(captured.stdout, bypassed.stdout)
            self.assertEqual(len(profiler.verify_child_output(root)), 1)
            # Nonempty unrelated output must not satisfy the capture requirement.
            for log in (root / "child-output").glob("*.jsonl"):
                log.write_text('{"Action":"start","Package":"other"}\n')
            with self.assertRaisesRegex(RuntimeError, "not captured"):
                profiler.verify_child_output(root)

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
