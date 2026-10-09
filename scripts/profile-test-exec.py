#!/usr/bin/python3
"""Restore the private capture wrapper after go test prepends GOROOT/bin."""

import os
from pathlib import Path
import sys


if len(sys.argv) < 2:
    raise SystemExit("test-exec requires the unchanged test binary and arguments")

os.environ["PATH"] = str(Path(__file__).resolve().parent) + os.pathsep + os.environ["PATH"]
os.execv(sys.argv[1], sys.argv[1:])
