#!/usr/bin/env python3
"""Pinned repository lint/format tools. Install explicitly; never alter system tools."""

from __future__ import annotations

import argparse
import hashlib
import io
import json
import os
import platform
import subprocess
import sys
import tarfile
import tempfile
import urllib.request
import zipfile
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
LOCK = json.loads((ROOT / "tools/quality.lock.json").read_text())
TOOLS = ROOT / ".tools/quality"
EXCLUDED = ("internal/dashboardassets/dist/", "test-results/", ".tools/", "web/node_modules/")
PRETTIER_SUFFIXES = {
    ".ts",
    ".js",
    ".mjs",
    ".vue",
    ".css",
    ".html",
    ".svg",
    ".json",
    ".jsonc",
    ".yml",
    ".yaml",
    ".md",
}


def run(arguments, *, cwd=ROOT):
    display = (
        arguments
        if len(arguments) <= 12
        else [*arguments[:4], f"... ({len(arguments) - 4} more arguments)"]
    )
    print("+ " + " ".join(str(value) for value in display), flush=True)
    subprocess.run(
        [str(value) for value in arguments],
        cwd=cwd,
        check=True,
        env={**os.environ, "POWERSHELL_TELEMETRY_OPTOUT": "1", "POWERSHELL_UPDATECHECK": "Off"},
    )


def files():
    output = subprocess.check_output(
        ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"], cwd=ROOT
    )
    return sorted(
        {
            name
            for raw in output.split(b"\0")
            if raw
            if (name := os.fsdecode(raw))
            and not name.startswith(EXCLUDED)
            and (ROOT / name).is_file()
        }
    )


def js(package, entry, *args, cwd=ROOT):
    path = ROOT / "web/node_modules" / package / entry
    if not path.is_file():
        raise SystemExit("Run bun install --cwd web --frozen-lockfile first")
    run(["bun", "--bun", path, *args], cwd=cwd)


def native(name):
    spec = LOCK["native"][name]
    path = TOOLS / f"{name}-{spec['version']}" / spec["executable"]
    if not path.is_file():
        raise SystemExit(f"Missing {name} {spec['version']}; run bun run tools:install")
    return path


def install_native(name, spec):
    destination = TOOLS / f"{name}-{spec['version']}"
    if (destination / spec["executable"]).is_file():
        return
    if destination.exists():
        raise SystemExit(f"Incomplete quality tool directory; inspect {destination}")
    cache = ROOT / ".tools/downloads"
    cache.mkdir(parents=True, exist_ok=True)
    archive = cache / spec["sha256"]
    if not archive.exists():
        print(f"Downloading pinned {name} {spec['version']} to .tools", flush=True)
        request = urllib.request.Request(
            spec["url"], headers={"User-Agent": "Velnix-quality-tools"}
        )
        with urllib.request.urlopen(request, timeout=90) as response:
            content = response.read(256 * 1024 * 1024 + 1)
        if len(content) > 256 * 1024 * 1024:
            raise SystemExit(f"{name} archive exceeds setup limit")
        if hashlib.sha256(content).hexdigest() != spec["sha256"]:
            raise SystemExit(f"{name} archive checksum mismatch")
        archive.write_bytes(content)
    content = archive.read_bytes()
    if hashlib.sha256(content).hexdigest() != spec["sha256"]:
        raise SystemExit(f"Cached {name} archive checksum mismatch; preserving it for inspection")
    TOOLS.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".install-", dir=TOOLS) as temporary:
        staging = Path(temporary) / "tool"
        staging.mkdir()
        if spec["kind"] == "ruff-wheel":
            with zipfile.ZipFile(io.BytesIO(content)) as wheel:
                matches = [n for n in wheel.namelist() if n.endswith(".data/scripts/ruff")]
                if len(matches) != 1:
                    raise SystemExit("Unexpected Ruff wheel executable layout")
                (staging / "ruff").write_bytes(wheel.read(matches[0]))
        elif spec["kind"] == "shellcheck-tar":
            with tarfile.open(fileobj=io.BytesIO(content)) as bundle:
                member = bundle.getmember(f"shellcheck-v{spec['version']}/shellcheck")
                source = bundle.extractfile(member)
                if source is None:
                    raise SystemExit("Missing ShellCheck executable")
                (staging / "shellcheck").write_bytes(source.read())
        elif spec["kind"] == "tar":
            with tarfile.open(fileobj=io.BytesIO(content)) as bundle:
                bundle.extractall(staging, filter="data")
        elif spec["kind"] == "zip":
            with zipfile.ZipFile(io.BytesIO(content)) as bundle:
                for member in bundle.infolist():
                    if not (staging / member.filename).resolve().is_relative_to(staging.resolve()):
                        raise SystemExit("Unsafe module archive path")
                bundle.extractall(staging)
        executable = staging / spec["executable"]
        if not executable.is_file():
            raise SystemExit(f"Missing expected {name} entry")
        if name != "psscriptanalyzer":
            executable.chmod(0o755)
        staging.rename(destination)


def powershell(mode):
    run(
        [
            native("powershell"),
            "-NoLogo",
            "-NoProfile",
            "-NonInteractive",
            "-File",
            ROOT / "scripts/quality-powershell.ps1",
            "-Mode",
            mode,
            "-AnalyzerPath",
            native("psscriptanalyzer"),
        ]
    )


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("action", choices=["install", "lint", "format"])
    parser.add_argument("--check", action="store_true")
    parser.add_argument("--fix", action="store_true")
    options = parser.parse_args()
    if options.check and options.action != "format" or options.fix and options.action != "lint":
        parser.error("--check is for format; --fix is for lint")
    if (
        subprocess.check_output(["bun", "--version"], text=True).strip()
        != (ROOT / ".bun-version").read_text().strip()
    ):
        raise SystemExit("Use the Bun version pinned in .bun-version")
    if options.action == "install":
        if sys.version_info < (3, 12):
            raise SystemExit("Quality setup requires Python 3.12+")
        if platform.system() != "Linux" or platform.machine() not in ("x86_64", "AMD64"):
            raise SystemExit(
                "Pinned native quality downloads currently support Linux x86_64 (including WSL); no system installation attempted"
            )
        for name, spec in LOCK["native"].items():
            install_native(name, spec)
        for name, package in LOCK["go"].items():
            run(["go", "run", package, "-version"])
        print("Quality tools ready in ignored .tools; Go uses its normal module/build cache")
        return
    names = files()
    go = [p for p in names if p.endswith(".go")]
    shell = [p for p in names if p.endswith(".sh")]
    python = [p for p in names if p.endswith(".py")]
    toml = [p for p in names if p.endswith(".toml")]
    if options.action == "format":
        if options.check:
            remaining = subprocess.check_output(["gofmt", "-s", "-l", *go], cwd=ROOT, text=True)
            if remaining:
                raise SystemExit("Go files need gofmt:\n" + remaining)
        else:
            run(["gofmt", "-s", "-w", *go])
        js(
            "prettier",
            "bin/prettier.cjs",
            "--check" if options.check else "--write",
            "--log-level",
            "warn",
            *[p for p in names if Path(p).suffix in PRETTIER_SUFFIXES],
        )
        run([native("ruff"), "format", *(["--check"] if options.check else []), *python])
        run(
            [
                "go",
                "run",
                LOCK["go"]["shfmt"],
                "-i",
                "2",
                "-ci",
                "-sr",
                "-d" if options.check else "-w",
                *shell,
            ]
        )
        js("@taplo/cli", "dist/cli.js", "format", *(["--check"] if options.check else []), *toml)
        powershell("Check" if options.check else "Format")
    else:
        run(["go", "vet", "./cmd/...", "./internal/..."])
        run(["go", "run", LOCK["go"]["staticcheck"], "./cmd/...", "./internal/..."])
        js(
            "eslint",
            "bin/eslint.js",
            ".",
            "--max-warnings",
            "0",
            *(["--fix"] if options.fix else []),
            cwd=ROOT / "web",
        )
        js(
            "stylelint",
            "bin/stylelint.mjs",
            "src/**/*.{css,vue}",
            "--allow-empty-input",
            "--max-warnings",
            "0",
            *(["--fix"] if options.fix else []),
            cwd=ROOT / "web",
        )
        run([native("ruff"), "check", *(["--fix"] if options.fix else []), *python])
        run([native("shellcheck"), "--severity=style", *shell])
        js("@taplo/cli", "dist/cli.js", "lint", "--no-schema", *toml)
        js(
            "markdownlint-cli2",
            "markdownlint-cli2-bin.mjs",
            *(["--fix"] if options.fix else []),
            *[p for p in names if p.endswith(".md")],
        )
        powershell("Lint")


if __name__ == "__main__":
    try:
        main()
    except subprocess.CalledProcessError as error:
        raise SystemExit(error.returncode) from None
