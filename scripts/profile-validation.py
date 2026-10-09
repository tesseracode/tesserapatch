#!/usr/bin/env python3
"""Profile the unchanged serial validation workload on macOS, without relaxing admission."""

import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import platform
import re
import shlex
import signal
import subprocess
import sys
import time


class AdmissionTimeout(RuntimeError):
    pass


class QuietWindow:
    def __init__(self):
        self.since = None
        self.previous = None

    def observe(self, sample):
        now = sample["monotonic"]
        healthy = (
            sample["admission_free_percent"] >= 80
            and sample["load1"] <= 5
            and not sample["go_processes"]
        )
        gap = self.previous is not None and (
            now < self.previous or now - self.previous > 5
        )
        if not healthy or gap:
            self.since = None
        if healthy and self.since is None:
            self.since = now
        self.previous = now
        return healthy and now - self.since >= 60


def output(*args, cwd=None):
    return subprocess.check_output(args, cwd=cwd, text=True, timeout=15)


def match(pattern, text):
    found = re.search(pattern, text, re.MULTILINE)
    if found is None:
        raise ValueError("required telemetry field not found: " + pattern)
    return found


def processes(text):
    rows = []
    for line in text.splitlines():
        pid, ppid, cpu, rss, command = line.split(None, 4)
        rows.append({
            "pid": int(pid), "ppid": int(ppid), "cpu_percent": float(cpu),
            "rss_bytes": int(rss) * 1024, "name": Path(command).name,
        })
    return rows


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
        "kern.memorystatus_vm_pressure_level",
    )
    vm = output("/usr/bin/vm_stat")
    rows = processes(output("/bin/ps", "-axo", "pid=,ppid=,%cpu=,rss=,comm="))
    page_size = int(match(r"page size of (\d+) bytes", vm)[1])
    pages = {}
    for name in (
        "Pages free", "Pages active", "Pages inactive", "Pages speculative",
        "Pages wired down", "Pages occupied by compressor", "Pageouts",
        "Swapins", "Swapouts",
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
        "go_processes": [
            p for p in rows if p["name"] in {"go", "compile", "link", "vet", "test"}
            or p["name"].endswith(".test")
        ],
        "owned_processes": tree,
        "owned_rss_sum_bytes": sum(p["rss_bytes"] for p in tree),
        "owned_cpu_percent_sum": sum(p["cpu_percent"] for p in tree),
    }


def record(stream, stage, phase, value):
    stream.write(json.dumps({"stage": stage, "phase": phase, **value}) + "\n")
    stream.flush()


def admit(stream, stage, wait_seconds, sampler=sample, clock=time.monotonic,
          sleep=time.sleep):
    start = clock()
    quiet = QuietWindow()
    while clock() - start <= wait_seconds:
        value = sampler()
        record(stream, stage, "admission", value)
        if quiet.observe(value):
            return clock() - start
        sleep(2)
    raise AdmissionTimeout("no continuous qualifying minute before " + stage)


def workload(repo, toolchain):
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
    return stages


def stop_owned(process):
    if process.poll() is None:
        # Every workload has a new session: its group never contains unrelated work.
        os.killpg(process.pid, signal.SIGTERM)
        try:
            process.wait(timeout=10)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.wait()


def execute(command, repo, env, log, stream, stage):
    start = time.monotonic()
    peak_rss = peak_count = peak_go = peak_pressure = 0
    lowest_free = 100
    first = last = None
    with log.open("x") as sink:
        child = subprocess.Popen(
            ["/usr/bin/time", "-l"] + command, cwd=repo, env=env,
            stdout=sink, stderr=subprocess.STDOUT, start_new_session=True,
        )
        try:
            while child.poll() is None:
                value = sample(child.pid)
                record(stream, stage, "execution", value)
                first = first or value
                last = value
                peak_rss = max(peak_rss, value["owned_rss_sum_bytes"])
                peak_count = max(peak_count, len(value["owned_processes"]))
                peak_go = max(peak_go, len(value["go_processes"]))
                peak_pressure = max(peak_pressure, value["pressure_level"])
                lowest_free = min(lowest_free, value["admission_free_percent"])
                if value["pressure_level"] not in (1, 2):
                    raise RuntimeError("critical or unknown OS pressure; stopped owned workload")
                time.sleep(2)
        finally:
            stop_owned(child)
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
        "sampled_peak_pressure_level": peak_pressure,
        "sampled_min_admission_free_percent": lowest_free,
        "swap_used_delta_bytes": last["swap_used_bytes"] - first["swap_used_bytes"] if first else None,
        "sampled_swapout_delta_pages": last["vm_pages"]["Swapouts"] - first["vm_pages"]["Swapouts"] if first else None,
    }
    if stage == "06-gofmt":
        result["formatted"] = not any(line.endswith(".go") for line in text.splitlines())
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--toolchain", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    parser.add_argument("--expected-head", required=True)
    parser.add_argument("--wait-seconds", type=int, default=600)
    args = parser.parse_args()
    if platform.system() != "Darwin" or args.wait_seconds < 60:
        parser.error("macOS and at least a 60-second admission budget required")

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
    stages = workload(repo, toolchain)
    destination.mkdir(parents=True, exist_ok=False)
    env = os.environ.copy()
    env.update({
        "PATH": str(toolchain / "bin") + os.pathsep + os.environ["PATH"],
        "BASH_ENV": "/dev/null", "GOFLAGS": "", "GOENV": "off", "GOMAXPROCS": "1",
        "GOTOOLCHAIN": "local", "GOWORK": "off",
        "GOPATH": str(destination / "cache/gopath"),
        "GOCACHE": str(destination / "cache/build"),
    })
    summary = {
        "status": "running", "head": head, "profiler_pid": os.getpid(),
        "toolchain_version_file": (toolchain / "VERSION").read_text().strip(),
        "architecture": platform.machine(), "os_release": platform.release(),
        "hardware": output("/usr/sbin/sysctl", "hw.memsize", "hw.physicalcpu", "hw.logicalcpu").strip(),
        "shard_script_sha256": hashlib.sha256((repo / "scripts/wave-close-test-shards.sh").read_bytes()).hexdigest(),
        "profiler_sha256": hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
        "initial_cache": "new isolated empty build/module caches; no global cache was cleared",
        "go_environment": {
            key: env[key] for key in (
                "GOENV", "GOFLAGS", "GOMAXPROCS", "GOTOOLCHAIN", "GOWORK",
            )
        },
        "admission": {"free_percent_min": 80, "load1_max": 5, "quiet_seconds": 60,
                      "no_existing_go_tools": True, "max_wait_seconds": args.wait_seconds},
        "limits": [
            "This profiles validation stages 6-8; it is not a new wave-close certification.",
            "Percent-free is the legacy admission metric, not Activity Monitor pressure.",
            "RSS sums double-count shared pages and miss processes between 2-second samples.",
            "time -l maximum RSS is not an aggregate simultaneous process-tree high-water mark.",
            "ps CPU percentages are OS-reported averages, not exact interval utilization.",
            "Cold isolated caches differ from the previous unprofiled successful run.",
        ],
        "results": [],
    }
    summary_file = destination / "summary.json"

    def save():
        summary_file.write_text(json.dumps(summary, indent=2) + "\n")

    save()
    try:
        with (destination / "samples.jsonl").open("x") as stream:
            for stage, command in stages:
                summary["current_stage"] = stage
                save()
                print("Waiting for unchanged admission: " + stage, flush=True)
                waited = admit(stream, stage, args.wait_seconds)
                print("Running: " + stage, flush=True)
                result = execute(command, repo, env, destination / (stage + ".log"), stream, stage)
                result["admission_wait_seconds"] = waited
                summary["results"].append(result)
                save()
                if result["exit_code"] or not result.get("formatted", True):
                    summary["status"] = "validation-failed"
                    return 1
            summary["status"] = "passed"
            return 0
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
