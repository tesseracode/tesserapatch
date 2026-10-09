#!/usr/bin/python3
"""Privately tee nested observer JSON bytes without changing Go arguments."""

import os
from pathlib import Path
import subprocess
import sys


def main():
    real_go = os.environ["TPATCH_PROFILE_REAL_GO"]
    args = sys.argv[1:]
    is_observer = (
        args[:1] == ["test"] and "-json" in args
        and any(arg.startswith("-overlay=") for arg in args)
    )
    if not is_observer:
        os.execv(real_go, [real_go] + args)
    path = Path(os.environ["TPATCH_PROFILE_CHILD_OUTPUT_DIR"]) / (
        "nested-go-%d.jsonl" % os.getpid()
    )
    # Fail before launching Go if the evidence destination cannot be opened.
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "wb", buffering=0) as evidence:
        child = subprocess.Popen([real_go] + args, stdout=subprocess.PIPE)
        try:
            while True:
                data = os.read(child.stdout.fileno(), 65536)
                if not data:
                    break
                remaining = memoryview(data)
                while remaining:
                    remaining = remaining[os.write(evidence.fileno(), remaining):]
                remaining = memoryview(data)
                while remaining:
                    remaining = remaining[os.write(sys.stdout.fileno(), remaining):]
            code = child.wait()
            return code if code >= 0 else 128 - code
        finally:
            child.stdout.close()
            if child.poll() is None:
                child.terminate()
                try:
                    child.wait(timeout=3)
                except subprocess.TimeoutExpired:
                    child.kill()
                    child.wait()


if __name__ == "__main__":
    sys.exit(main())
