#!/usr/bin/env python3
"""Build frontend assets twice and verify reproducibility and size."""

import gzip
import subprocess
from pathlib import Path

root = Path(__file__).resolve().parent.parent
assets = root / "internal/dashboardassets/dist"


def snapshot():
    return {str(p.relative_to(assets)): p.read_bytes() for p in assets.rglob("*") if p.is_file()}


assert (
    subprocess.check_output(["bun", "--version"], text=True).strip()
    == (root / ".bun-version").read_text().strip()
), "Bun version differs from pin"
subprocess.run(["bun", "run", "--cwd", "web", "build"], cwd=root, check=True)
before = snapshot()
assert "index.html" in before and any(p.endswith(".js") for p in before), (
    "Missing production dashboard assets"
)
subprocess.run(["bun", "run", "--cwd", "web", "build"], cwd=root, check=True)
after = snapshot()
assert before == after, "Dashboard assets differ between consecutive production builds"
size = sum(
    len(gzip.compress(value, mtime=0))
    for name, value in after.items()
    if name.endswith((".js", ".css"))
)
assert size <= 250 * 1024, f"Compressed JS/CSS exceeds 250 KiB budget: {size}"
print(f"Dashboard assets reproduce exactly; compressed JS/CSS: {size} bytes")
