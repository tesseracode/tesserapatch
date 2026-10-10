#!/usr/bin/env python3
"""Profile serial macOS validation with explicitly selected experimental safeguards."""

import argparse
import datetime
import hashlib
import json
import math
import os
from pathlib import Path
import platform
import re
import shlex
import shutil
import signal
import subprocess
import sys
import time


class AdmissionTimeout(RuntimeError):
    pass


class BudgetExceeded(RuntimeError):
    pass


class ResourceStop(RuntimeError):
    pass


class Budget:
    def __init__(self, seconds=None):
        self.deadline = None if seconds is None else time.monotonic() + seconds

    def timeout(self, maximum=15):
        if self.deadline is None:
            return maximum
        remaining = self.deadline - time.monotonic()
        if remaining <= 0:
            raise BudgetExceeded("overall experiment budget exhausted")
        return min(maximum, remaining)

    def pause(self, seconds):
        time.sleep(self.timeout(seconds))


BUDGET = Budget()


class QuietWindow:
    def __init__(self, minimum_free=80):
        if minimum_free not in (75, 80):
            raise ValueError("only default 80% or explicitly selected 75% admission supported")
        self.minimum_free = minimum_free
        self.since = None
        self.previous = None

    def healthy(self, sample):
        return (
            sample["admission_free_percent"] >= self.minimum_free
            and sample["load1"] <= 5
            and not sample["go_processes"]
        )

    def observe(self, sample):
        healthy = self.healthy(sample)
        now = sample["monotonic"]
        gap = self.previous is not None and (
            now < self.previous or now - self.previous > 5
        )
        if not healthy or gap:
            self.since = None
        if healthy and self.since is None:
            self.since = now
        self.previous = now
        return healthy and now - self.since >= 60


def pressure_counters(value):
    try:
        level = value["pressure_level"]
        swapouts = value["vm_pages"]["Swapouts"]
        timestamp = value["monotonic"]
        load = value["load1"]
        go_processes = value["go_processes"]
    except (KeyError, TypeError) as error:
        raise ValueError("pressure policy requires OS pressure and swap-out telemetry") from error
    if type(level) is not int or level not in (1, 2, 4):
        raise ValueError("unrecognized OS memory pressure level")
    if type(swapouts) is not int or swapouts < 0:
        raise ValueError("invalid cumulative swap-out counter")
    if any(type(item) not in (int, float) or not math.isfinite(item)
           for item in (timestamp, load)) or load < 0 or not isinstance(go_processes, list):
        raise ValueError("pressure policy requires valid timing, load and process telemetry")
    return level, swapouts


class PressureWindow(QuietWindow):
    def __init__(self, allow_external_go=False):
        super().__init__()
        self.allow_external_go = allow_external_go
        self.swapouts = None
        self.latest = None

    def healthy(self, sample):
        level, swapouts = pressure_counters(sample)
        if self.swapouts is not None and swapouts < self.swapouts:
            raise ValueError("cumulative swap-out counter moved backwards")
        unchanged = self.swapouts is None or swapouts == self.swapouts
        self.swapouts = swapouts
        self.latest = sample
        return (level == 1 and unchanged and sample["load1"] <= 5
                and (self.allow_external_go or not sample["go_processes"]))


def swap_usage(value):
    usage = value.get("swap_used_bytes")
    if type(usage) is not int or usage < 0:
        raise ValueError("swap-tolerant policy requires valid occupied-swap telemetry")
    return usage


def require_runtime_pressure_safe(value, previous, max_swap_growth_bytes=0,
                                 swap_used_at_start=None):
    level, swapouts = pressure_counters(value)
    _, previous_swapouts = pressure_counters(previous)
    if swapouts < previous_swapouts:
        raise ValueError("cumulative swap-out counter moved backwards")
    if level != 1:
        raise ResourceStop("OS memory pressure is no longer normal; stopped owned workload")
    if max_swap_growth_bytes:
        if type(swap_used_at_start) is not int or swap_used_at_start < 0:
            raise ValueError("swap-tolerant policy requires a valid prelaunch baseline")
        if swap_usage(value) - swap_used_at_start > max_swap_growth_bytes:
            raise ResourceStop("occupied swap grew beyond the experimental limit; stopped owned workload")
    elif swapouts > previous_swapouts:
        raise ResourceStop("new system swap-outs observed; stopped owned workload")
    gap = value["monotonic"] - previous["monotonic"]
    if gap < 0 or gap > 5:
        raise ResourceStop("runtime monitoring gap exceeds the safe sampling window")


def output(*args, cwd=None):
    try:
        return subprocess.check_output(
            args, cwd=cwd, text=True, timeout=BUDGET.timeout(),
            env={**os.environ, "LC_ALL": "C"},
        )
    except subprocess.TimeoutExpired:
        BUDGET.timeout()
        raise


def match(pattern, text):
    found = re.search(pattern, text, re.MULTILINE)
    if found is None:
        raise ValueError("required telemetry field not found: " + pattern)
    return found


def cpu_seconds(text):
    days, separator, rest = text.partition("-")
    total = int(days) * 86400 if separator else 0
    fields = (rest if separator else text).split(":")
    if len(fields) not in (2, 3):
        raise ValueError("unexpected ps CPU-time format: " + text)
    return total + sum(float(part) * 60 ** index
                       for index, part in enumerate(reversed(fields)))


def processes(text):
    rows = []
    for line in text.splitlines():
        fields = line.split(None, 10)
        if len(fields) != 11:
            raise ValueError("incomplete ps identity/CPU-time row")
        pid, ppid, cpu, rss, cumulative = fields[:5]
        started = " ".join(fields[5:10])
        rows.append({
            "pid": int(pid), "ppid": int(ppid), "cpu_percent": float(cpu),
            "rss_bytes": int(rss) * 1024, "name": Path(fields[10]).name,
            "cpu_time_seconds": cpu_seconds(cumulative), "started_local": started,
            "start_epoch": time.mktime(time.strptime(started, "%a %b %d %H:%M:%S %Y")),
        })
    return rows


class ProcessTracker:
    def __init__(self):
        self.previous = {}
        self.when = None

    def annotate(self, rows, now, boot_epoch):
        elapsed = None if self.when is None else now - self.when
        parents = {p["pid"]: p["name"] for p in rows}
        for process in rows:
            identity = (process["pid"], process["started_local"], process["name"])
            old = self.previous.get(identity)
            delta = None if old is None else process["cpu_time_seconds"] - old
            process["interval_cpu_percent"] = (
                100 * delta / elapsed
                if delta is not None and delta >= 0 and elapsed is not None and elapsed > 0
                else None
            )
            process["parent_name"] = parents.get(process["ppid"])
            process["seconds_after_boot_started"] = process["start_epoch"] - boot_epoch
        self.previous = {
            (p["pid"], p["started_local"], p["name"]): p["cpu_time_seconds"] for p in rows
        }
        self.when = now


PROCESS_TRACKER = ProcessTracker()


def descendants(rows, root):
    selected = {root}
    while True:
        expanded = selected | {p["pid"] for p in rows if p["ppid"] in selected}
        if selected == expanded:
            return [p for p in rows if p["pid"] in selected]
        selected = expanded


def sample(root=None):
    begin = time.monotonic()
    pressure = output("/usr/bin/memory_pressure", "-Q")
    kernel = output(
        "/usr/sbin/sysctl", "vm.loadavg", "vm.swapusage",
        "kern.memorystatus_vm_pressure_level", "kern.boottime",
    )
    vm = output("/usr/bin/vm_stat")
    rows = processes(output("/bin/ps", "-axo", "pid=,ppid=,%cpu=,rss=,time=,lstart=,comm="))
    boot_epoch = int(match(r"kern.boottime:\s+\{\s+sec = (\d+)", kernel)[1])
    PROCESS_TRACKER.annotate(rows, time.monotonic(), boot_epoch)
    page_size = int(match(r"page size of (\d+) bytes", vm)[1])
    pages = {}
    for name in (
        "Pages free", "Pages active", "Pages inactive", "Pages speculative",
        "Pages wired down", "Pages occupied by compressor", "Pageouts",
        "Swapins", "Swapouts", "Pages purgeable", "File-backed pages",
        "Anonymous pages", "Pages stored in compressor", "Compressions", "Decompressions",
    ):
        pages[name] = int(match(r"^" + re.escape(name) + r":\s+(\d+)\.", vm)[1])
    tree = descendants(rows, root) if root is not None else []
    return {
        "utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
        "monotonic": time.monotonic(),
        "sample_duration_seconds": time.monotonic() - begin,
        "admission_free_percent": int(match(r"free percentage:\s+(\d+)%", pressure)[1]),
        "load1": float(match(r"vm.loadavg:\s+\{\s+([\d.]+)", kernel)[1]),
        "pressure_level": int(match(r"pressure_level:\s+(\d+)", kernel)[1]),
        "swap_used_bytes": int(float(match(r"used = ([\d.]+)M", kernel)[1]) * 1024**2),
        "page_size_bytes": page_size,
        "vm_pages": pages,
        "system_process_count": len(rows),
        "uptime_seconds": time.time() - boot_epoch,
        "top_rss_processes": sorted(rows, key=lambda p: (-p["rss_bytes"], p["pid"]))[:12],
        "top_interval_cpu_processes": sorted(
            [p for p in rows if p["interval_cpu_percent"] is not None],
            key=lambda p: (-p["interval_cpu_percent"], p["pid"]),
        )[:12],
        "go_processes": [
            p for p in rows if p["name"] in {"go", "compile", "link", "vet", "test"}
            or p["name"].endswith(".test")
        ],
        "owned_processes": tree,
        "owned_rss_sum_bytes": sum(p["rss_bytes"] for p in tree),
        "owned_cpu_percent_sum": sum(p["cpu_percent"] for p in tree),
        "owned_interval_cpu_percent_sum": sum(
            p["interval_cpu_percent"] for p in tree if p["interval_cpu_percent"] is not None
        ),
    }


def record(stream, stage, phase, value):
    stream.write(json.dumps({"stage": stage, "phase": phase, **value}) + "\n")
    stream.flush()


def admit(stream, stage, wait_seconds, sampler=sample, clock=time.monotonic,
          sleep=time.sleep, minimum_free=80, window=None):
    start = clock()
    quiet = window if window is not None else QuietWindow(minimum_free)
    while clock() - start <= wait_seconds:
        BUDGET.timeout()
        value = sampler()
        record(stream, stage, "admission", value)
        BUDGET.timeout()
        if quiet.observe(value):
            return clock() - start
        sleep(BUDGET.timeout(2))
    raise AdmissionTimeout("no continuous qualifying minute before " + stage)


def workload(repo, toolchain, profile="full"):
    preamble = (
        "#!/bin/sh\nset -eu\n\nBASH_ENV=/dev/null\nGOFLAGS=\nGOENV=off\n"
        "GOMAXPROCS=1\nexport BASH_ENV GOFLAGS GOENV GOMAXPROCS\n\n"
    )
    body = (repo / "scripts/wave-close-test-shards.sh").read_text()
    if not body.startswith(preamble):
        raise ValueError("shard environment changed; recheck profiling command parity")
    commands = [shlex.split(line) for line in body[len(preamble):].splitlines()]
    if len(commands) != 22 or any(
        command[:3] != ["go", "test", "-p=1"]
        or "-count=1" not in command or command[command.index("-timeout") + 1] != "40m"
        for command in commands
    ):
        raise ValueError("expected unchanged 22-command serial shard partition")
    go = str(toolchain / "bin/go")
    stages = [
        ("06-gofmt", [str(toolchain / "bin/gofmt"), "-l", "."]),
        ("07-vet", [go, "vet", "-p=1", "./..."]),
        ("07-build", [go, "build", "-p=1", "./cmd/tpatch"]),
    ]
    stages.extend(
        ("08-shard-%02d" % (i + 1), [go] + command[1:])
        for i, command in enumerate(commands)
    )
    if profile == "full":
        return stages
    if profile == "partition":
        return stages[3:]
    if profile == "broad":
        return stages[3:4]
    if profile == "ap-observer":
        return [(name, command) for name, command in stages if name == "08-shard-15"]
    if profile != "pilot":
        raise ValueError("unknown workload profile")
    selected = {"06-gofmt", "07-vet", "07-build", "08-shard-02", "08-shard-15"}
    return [(name, command) for name, command in stages if name in selected]


def seed_cache(source, destination, head, version):
    source = source.resolve()
    raw = (source / "summary.json").read_bytes()
    previous = json.loads(raw)
    if previous["head"] != head or previous["toolchain_version_file"] != version:
        raise ValueError("cache source must match frozen HEAD and toolchain")
    if previous["status"] in ("running", "interrupted"):
        raise ValueError("cache source must be a completed run")
    cache = source / "cache"
    if not (cache / "build").is_dir() or not (cache / "gopath").is_dir():
        raise ValueError("completed run has no isolated build/module caches")
    shutil.copytree(cache, destination / "cache")
    return {
        "source_run": source.name,
        "source_summary_sha256": hashlib.sha256(raw).hexdigest(),
        "initial_cache": "copy of previous completed isolated caches; original caches unchanged",
    }


def capture_child_output(destination, toolchain, env):
    helper = Path(__file__).with_name("profile-go-output.py")
    body = helper.read_bytes()
    wrapper_dir = destination / "wrappers"
    output_dir = destination / "child-output"
    wrapper_dir.mkdir(mode=0o700)
    output_dir.mkdir(mode=0o700)
    wrapper = wrapper_dir / "go"
    wrapper.write_bytes(body)
    wrapper.chmod(0o700)
    exec_body = Path(__file__).with_name("profile-test-exec.py").read_bytes()
    test_exec = wrapper_dir / "test-exec"
    test_exec.write_bytes(exec_body)
    test_exec.chmod(0o700)
    env.update({
        "PATH": str(wrapper_dir) + os.pathsep + env["PATH"],
        "TPATCH_PROFILE_REAL_GO": str(toolchain / "bin/go"),
        "TPATCH_PROFILE_CHILD_OUTPUT_DIR": str(output_dir),
    })
    return {
        "child_output_capture_sha256": hashlib.sha256(body).hexdigest(),
        "test_exec_capture_sha256": hashlib.sha256(exec_body).hexdigest(),
    }


def verify_child_output(destination):
    paths = sorted((destination / "child-output").glob("*.jsonl"))
    for path in paths:
        with path.open() as stream:
            for line in stream:
                event = json.loads(line)
                if (event.get("Action") == "run"
                        and event.get("Package") == "github.com/tesseracode/tesserapatch/internal/cli"
                        and event.get("Test") == "TestS7APDryRunContracts"):
                    return [p.name for p in paths]
    raise RuntimeError("nested observer output was not captured; diagnostic run is incomplete")


def stop_owned(process, known=()):
    try:
        stop_owned_tree(process, known)
    finally:
        if process.poll() is None:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait(timeout=3)


def stop_owned_tree(process, known):
    if process.poll() is None:
        def live_processes():
            return processes(subprocess.check_output(
                ["/bin/ps", "-axo", "pid=,ppid=,%cpu=,rss=,time=,lstart=,comm="],
                text=True, timeout=3, env={**os.environ, "LC_ALL": "C"},
            ))

        def identity(p):
            return p["pid"], p["started_local"], p["name"]

        rows = live_processes()
        owned = {identity(p) for p in known}
        owned.update(identity(p) for p in descendants(rows, process.pid))

        def signal_children(current, sig):
            for p in current:
                if p["pid"] != process.pid and identity(p) in owned:
                    try:
                        os.kill(p["pid"], sig)
                    except ProcessLookupError:
                        pass

        # Observer tests create separate groups; the outer group alone is insufficient.
        os.killpg(process.pid, signal.SIGTERM)
        signal_children(rows, signal.SIGTERM)
        end = time.monotonic() + 10
        while time.monotonic() < end:
            process.poll()
            rows = live_processes()
            if not any(identity(p) in owned for p in rows):
                return
            time.sleep(0.1)
        if process.poll() is None:
            os.killpg(process.pid, signal.SIGKILL)
        signal_children(rows, signal.SIGKILL)
        process.wait(timeout=3)


def execute(command, repo, env, log, stream, stage, pressure_baseline=None,
            allow_external_go=False, max_swap_growth_bytes=0):
    BUDGET.timeout()
    if max_swap_growth_bytes and pressure_baseline is None:
        raise ValueError("swap tolerance requires pressure-based monitoring")
    start = time.monotonic()
    peak_rss = peak_count = peak_go = peak_pressure = 0
    peak_external_go = 0
    lowest_free = 100
    first = last = None
    owned = {}
    previous_pressure = pressure_baseline
    swap_used_at_start = None
    peak_swap_used = 0
    if pressure_baseline is not None:
        first = last = sample()
        record(stream, stage, "prelaunch", first)
        require_runtime_pressure_safe(first, pressure_baseline)
        if first["load1"] > 5 or (first["go_processes"] and not allow_external_go):
            raise ResourceStop("admission conditions changed before launching the workload")
        if max_swap_growth_bytes:
            swap_used_at_start = swap_usage(first)
            peak_swap_used = swap_used_at_start
        previous_pressure = first
    with log.open("x") as sink:
        child = subprocess.Popen(
            ["/usr/bin/time", "-l"] + command, cwd=repo, env=env,
            stdout=sink, stderr=subprocess.STDOUT, start_new_session=True,
        )
        try:
            while child.poll() is None:
                BUDGET.timeout()
                value = sample(child.pid)
                record(stream, stage, "execution", value)
                owned.update((p["pid"], p) for p in value["owned_processes"])
                first = first or value
                last = value
                peak_rss = max(peak_rss, value["owned_rss_sum_bytes"])
                peak_count = max(peak_count, len(value["owned_processes"]))
                peak_go = max(peak_go, len(value["go_processes"]))
                owned_pids = {p["pid"] for p in value["owned_processes"]}
                peak_external_go = max(peak_external_go, sum(
                    p["pid"] not in owned_pids for p in value["go_processes"]
                ))
                peak_pressure = max(peak_pressure, value["pressure_level"])
                lowest_free = min(lowest_free, value["admission_free_percent"])
                if pressure_baseline is not None:
                    require_runtime_pressure_safe(value, previous_pressure,
                                                  max_swap_growth_bytes, swap_used_at_start)
                    if max_swap_growth_bytes:
                        peak_swap_used = max(peak_swap_used, swap_usage(value))
                    previous_pressure = value
                elif value["pressure_level"] not in (1, 2):
                    raise RuntimeError("critical or unknown OS pressure; stopped owned workload")
                BUDGET.pause(2)
            if pressure_baseline is not None:
                last = sample(child.pid)
                record(stream, stage, "completion", last)
                require_runtime_pressure_safe(last, previous_pressure,
                                              max_swap_growth_bytes, swap_used_at_start)
                if max_swap_growth_bytes:
                    peak_swap_used = max(peak_swap_used, swap_usage(last))
        finally:
            stop_owned(child, owned.values())
    text = log.read_text()
    timing = re.search(r"^\s*([\d.]+)\s+real\s+([\d.]+)\s+user\s+([\d.]+)\s+sys\s*$",
                       text, re.MULTILINE)
    rss = re.search(r"^\s*(\d+)\s+maximum resident set size\s*$", text, re.MULTILINE)
    if timing is None or rss is None:
        raise ValueError("time -l measurements absent; see " + str(log))
    result = {
        "stage": stage, "command": command, "exit_code": child.returncode,
        "measured_elapsed_seconds": time.monotonic() - start,
        "time_real_seconds": float(timing[1]), "time_user_seconds": float(timing[2]),
        "time_sys_seconds": float(timing[3]), "time_reported_max_rss_bytes": int(rss[1]),
        "sampled_peak_owned_rss_sum_bytes": peak_rss,
        "sampled_peak_owned_process_count": peak_count,
        "sampled_peak_system_go_process_count": peak_go,
        "sampled_peak_external_go_process_count": peak_external_go,
        "sampled_peak_pressure_level": peak_pressure,
        "sampled_min_admission_free_percent": lowest_free,
        "swap_used_delta_bytes": last["swap_used_bytes"] - first["swap_used_bytes"] if first else None,
        "sampled_swapout_delta_pages": last["vm_pages"]["Swapouts"] - first["vm_pages"]["Swapouts"] if first else None,
    }
    if stage == "06-gofmt":
        result["formatted"] = not any(line.endswith(".go") for line in text.splitlines())
    if max_swap_growth_bytes:
        result.update(
            swap_used_at_start_bytes=swap_used_at_start,
            sampled_peak_swap_used_bytes=peak_swap_used,
            sampled_peak_swap_growth_bytes=peak_swap_used - swap_used_at_start,
        )
    return result


def main():
    global BUDGET
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--toolchain", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--expected-head", required=True)
    parser.add_argument("--wait-seconds", type=int, default=600)
    parser.add_argument("--memory-threshold", type=int, choices=(80, 75), default=80)
    parser.add_argument("--profile", choices=("full", "partition", "broad", "pilot", "ap-observer"), default="full")
    parser.add_argument("--admission-policy", choices=("percent", "pressure"), default="percent")
    parser.add_argument("--allow-external-go", action="store_true",
                        help="pressure-policy experiment only: observe other Go tools without vetoing admission")
    parser.add_argument("--max-seconds", type=int)
    parser.add_argument("--max-swap-growth-mib", type=int, choices=(0, 2048), default=0,
                        help="broad pressure probe only: maximum additional occupied swap, not cumulative traffic")
    parser.add_argument("--cache-from", type=Path)
    args = parser.parse_args()
    if platform.system() != "Darwin" or args.wait_seconds < 60:
        parser.error("macOS and at least a 60-second admission budget required")
    if args.max_seconds is not None and args.max_seconds < 60:
        parser.error("overall budget must be at least 60 seconds")
    if args.profile not in ("full", "partition") and args.max_seconds is None:
        parser.error("partial experiment requires an explicit overall time budget")
    if args.memory_threshold == 75 and (args.profile in ("full", "partition") or args.max_seconds > 1800):
        parser.error("75% experiment requires a partial profile and at most 1800 seconds")
    if args.cache_from is not None and args.profile != "ap-observer":
        parser.error("cache copying is only supported for the diagnostic AP repeat")
    if args.admission_policy == "pressure" and (
        args.profile not in ("partition", "broad") or args.memory_threshold != 80
    ):
        parser.error("pressure policy is an explicit partition experiment, not a percentage override")
    if args.allow_external_go and args.admission_policy != "pressure":
        parser.error("external Go permission requires the explicit pressure-policy experiment")
    if args.max_swap_growth_mib and (
        args.admission_policy != "pressure" or args.profile != "broad"
        or args.max_seconds is None or args.max_seconds > 3600
    ):
        parser.error("swap tolerance requires a pressure-based broad probe capped at 3600 seconds")

    def interrupt(signum, frame):
        raise KeyboardInterrupt

    signal.signal(signal.SIGTERM, interrupt)
    signal.signal(signal.SIGHUP, interrupt)
    repo, toolchain, destination = args.repo.resolve(), args.toolchain.resolve(), args.out.resolve()
    head = output("git", "rev-parse", "HEAD", cwd=repo).strip()
    if head != args.expected_head or output("git", "status", "--porcelain", cwd=repo):
        raise RuntimeError("baseline checkout must match expected HEAD and be clean")
    if repo == destination or repo in destination.parents:
        raise RuntimeError("keep telemetry/cache outside the frozen checkout")
    stages = workload(repo, toolchain, args.profile)
    destination.mkdir(parents=True, exist_ok=False)
    env = os.environ.copy()
    env.update({
        "PATH": str(toolchain / "bin") + os.pathsep + os.environ["PATH"],
        "BASH_ENV": "/dev/null", "GOFLAGS": "", "GOENV": "off", "GOMAXPROCS": "1",
        "GOTOOLCHAIN": "local", "GOWORK": "off",
        "GOPATH": str(destination / "cache/gopath"),
        "GOCACHE": str(destination / "cache/build"),
    })
    version = (toolchain / "VERSION").read_text().strip()
    cache_info = {"initial_cache": "new isolated empty build/module caches; no global cache was cleared"}
    if args.cache_from is not None:
        cache_info = seed_cache(args.cache_from, destination, head, version)
    capture_info = {}
    if args.profile == "ap-observer":
        capture_info = capture_child_output(destination, toolchain, env)
        stages = [(name, cmd[:2] + ["-exec", str(destination / "wrappers/test-exec")] + cmd[2:])
                  for name, cmd in stages]
    summary = {
        "status": "running", "head": head, "profiler_pid": os.getpid(),
        "toolchain_version_file": version,
        "architecture": platform.machine(), "os_release": platform.release(),
        "hardware": output("/usr/sbin/sysctl", "hw.memsize", "hw.physicalcpu", "hw.logicalcpu").strip(),
        "shard_script_sha256": hashlib.sha256((repo / "scripts/wave-close-test-shards.sh").read_bytes()).hexdigest(),
        "profiler_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        **cache_info,
        **capture_info,
        "profile": args.profile,
        "planned_stages": [name for name, _ in stages],
        "max_workload_seconds": args.max_seconds,
        "boot_time": output("/usr/sbin/sysctl", "-n", "kern.boottime").strip(),
        "go_environment": {
            key: env[key] for key in (
                "GOENV", "GOFLAGS", "GOMAXPROCS", "GOTOOLCHAIN", "GOWORK",
            )
        },
        "admission": {"policy": args.admission_policy,
                      "free_percent_min": args.memory_threshold if args.admission_policy == "percent" else None,
                      "normal_pressure_required": args.admission_policy == "pressure",
                      "no_new_swapouts_required": args.admission_policy == "pressure",
                      "load1_max": 5, "quiet_seconds": 60,
                      "no_existing_go_tools": not args.allow_external_go,
                      "max_wait_seconds": args.wait_seconds},
        "external_go_policy": "observe-only" if args.allow_external_go else "block-admission",
        "runtime_pressure_guard": args.admission_policy == "pressure",
        "runtime_swap_growth_limit_bytes": args.max_swap_growth_mib * 1024**2,
        "limits": [
            "This profiles validation stages 6-8; it is not a new wave-close certification.",
            "Percent-free is the legacy admission metric, not Activity Monitor pressure.",
            "RSS sums double-count shared pages and miss processes between 2-second samples.",
            "time -l maximum RSS is not an aggregate simultaneous process-tree high-water mark.",
            "ps CPU percentages are OS-reported averages, not exact interval utilization.",
            "Interval CPU uses cumulative CPU-time differences; 100% represents one logical CPU.",
            "New/reused process identities have no interval CPU estimate until the next sample.",
            "Process start timing near boot does not prove waste or authorize disabling a service.",
            "Process inventories remain local; argv, environment and full executable paths are not collected.",
            "Pilot covers only its five selected stages; budget exhaustion is not a test assertion failure.",
            "AP-only profile is diagnostic; a partial run cannot certify the whole suite.",
            "AP child-output tee preserves bytes/arguments but adds instrumentation overhead.",
            "AP top-level go test adds -exec solely to restore the capture PATH; test binary arguments are unchanged.",
            "Cache state must be considered when comparing timing between runs.",
            "Raw child test output may contain generated paths and tokens; retain it privately.",
            "Pressure policy is opt-in and host/run-specific; percentage is diagnostic only in that mode.",
            "New swap-outs are measured system-wide and may be caused by unrelated processes.",
            "Historical swap usage alone is not active swapping; compare cumulative swap-out counters.",
            "Pressure-policy runtime checks stop on warning/critical pressure or monitoring gaps.",
            "By default any new swap-out stops the workload. The explicit broad probe instead caps occupied swap growth from prelaunch.",
            "The swap-growth cap is system-wide, sampled and experimental, not an OS memory limit or a bound on cumulative swap traffic.",
            "Sampling may observe a limit after it has already been exceeded; no permanent swap policy is adopted.",
            "External Go permission changes only admission/prelaunch presence checks; load and memory safety remain enforced.",
            "Concurrent work can affect timing; frozen checkouts and isolated caches do not remove CPU/disk contention.",
        ],
        "results": [],
    }
    summary_file = destination / "summary.json"

    def save():
        summary_file.write_text(json.dumps(summary, indent=2) + "\n")

    save()
    BUDGET = Budget(args.max_seconds)
    try:
        with (destination / "samples.jsonl").open("x") as stream:
            for stage, command in stages:
                summary["current_stage"] = stage
                save()
                pressure_window = (PressureWindow(allow_external_go=args.allow_external_go)
                                   if args.admission_policy == "pressure" else None)
                label = "normal pressure/no swap-outs" if pressure_window is not None else "%d%%" % args.memory_threshold
                print("Waiting for %s admission: %s" % (label, stage), flush=True)
                waited = admit(stream, stage, args.wait_seconds,
                               minimum_free=args.memory_threshold, window=pressure_window)
                print("Running: " + stage, flush=True)
                result = execute(command, repo, env, destination / (stage + ".log"), stream, stage,
                                 pressure_baseline=pressure_window.latest if pressure_window is not None else None,
                                 allow_external_go=args.allow_external_go,
                                 max_swap_growth_bytes=args.max_swap_growth_mib * 1024**2)
                result["admission_wait_seconds"] = waited
                summary["results"].append(result)
                save()
                if args.profile == "ap-observer":
                    summary["captured_child_output_files"] = verify_child_output(destination)
                    save()
                if result["exit_code"] or not result.get("formatted", True):
                    summary["status"] = "validation-failed"
                    return 1
            summary["status"] = "passed" if args.profile == "full" else args.profile + "-passed"
            return 0
    except ResourceStop as error:
        summary.update(status="resource-stopped", error=str(error))
        return 76
    except BudgetExceeded as error:
        summary.update(status="budget-exhausted", error=str(error))
        return 124
    except AdmissionTimeout as error:
        summary.update(status="admission-blocked", error=str(error))
        return 75
    except (OSError, ValueError, RuntimeError, subprocess.SubprocessError) as error:
        summary.update(status="measurement-or-safety-error", error=str(error))
        return 1
    except KeyboardInterrupt:
        summary.update(status="interrupted")
        return 130
    finally:
        save()
        print("Result: " + summary["status"] + "; " + str(summary_file), flush=True)


if __name__ == "__main__":
    sys.exit(main())
