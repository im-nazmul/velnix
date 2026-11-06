#!/usr/bin/env python3
"""Disposable PTY acceptance for reviewed TUI registry mutations."""

import fcntl
import json
import os
import pty
import select
import struct
import subprocess
import sys
import tempfile
import termios
import time
from pathlib import Path


def validate(binary):
    with tempfile.TemporaryDirectory(prefix="velnix-tui-manage-") as directory:
        root = Path(directory)
        project = root / "project with spaces"
        project.mkdir()
        (project / "keep.txt").write_text("external source remains intact")
        env = dict(os.environ, TERM="xterm-256color", NO_COLOR="1")
        for kind in ["CONFIG", "DATA", "STATE", "CACHE"]:
            env[f"XDG_{kind}_HOME"] = str(root / kind.lower())
        registry = root / "data/velnix/registry.json"
        master, slave = pty.openpty()
        before = termios.tcgetattr(slave)
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 28, 100, 0, 0))
        child = subprocess.Popen([binary, "tui"], stdin=slave, stdout=slave, stderr=slave, env=env)
        output = bytearray()

        def send(data):
            os.write(master, data)

        def wait(text, timeout=8):
            deadline = time.monotonic() + timeout
            while time.monotonic() < deadline:
                if select.select([master], [], [], 0.05)[0]:
                    output.extend(os.read(master, 65536))
                if text.encode() in output:
                    output.clear()
                    return
            raise AssertionError(f"missing terminal output: {text}; {output[-3000:]!r}")

        def action(name):
            send(b"\x10")
            wait("What would you like to do?")
            send(name.encode())
            # Wait for the search frame before submitting the highlighted item.
            wait(name)
            send(b"\r")

        def approve(expected="Change applied.", preview_ready=False):
            if not preview_ready:
                wait("Read to the end")
            send(b"\x1b[F")
            wait("Type APPLY")
            send(b"APPLY")
            wait("APPLY")
            send(b"\r")
            wait(expected)
            send(b"\r")
            wait("v e l n i x")

        def link_form():
            action("Link project")
            wait("Absolute project path")
            send(b"one\r")
            send(str(project).encode() + b"\r")
            send(b"\r")  # Default document root '.'; preview only.

        def competing_change(action, name, path=None):
            args = [binary, "projects", action, "--name", name]
            if path is not None:
                args += ["--path", str(path)]
            planned = subprocess.run(
                args + ["--json"], env=env, check=True, capture_output=True, text=True
            )
            plan_id = json.loads(planned.stdout)["id"]
            subprocess.run(
                args + ["--approve", plan_id, "--json"], env=env, check=True, capture_output=True
            )

        try:
            wait("v e l n i x")
            action("Initialize state")
            assert not registry.exists(), "preview initialized state"
            approve()
            assert json.loads(registry.read_text()).get("projects", []) == []
            link_form()
            wait("Read to the end")
            assert not json.loads(registry.read_text()).get("projects"), "preview linked project"
            send(b"\x1b")
            wait("v e l n i x")
            assert not json.loads(registry.read_text()).get("projects"), "cancel applied plan"
            link_form()
            wait("Read to the end")
            # Leave the TUI preview open while another approved CLI operation
            # changes the same registry. The old approval must be refused.
            other = root / "other"
            other.mkdir()
            competing_change("link", "other", other)
            approve("Change could not complete.", preview_ready=True)
            assert [p["name"] for p in json.loads(registry.read_text())["projects"]] == ["other"]
            competing_change("unlink", "other")
            link_form()
            approve()
            assert json.loads(registry.read_text())["projects"][0]["name"] == "one"
            action("Unlink project")
            wait("Project name")
            send(b"one\r")
            approve()
            assert not json.loads(registry.read_text()).get("projects"), "unlink not applied"
            assert (project / "keep.txt").read_text() == "external source remains intact"
            send(b"q")
            child.wait(timeout=8)
            assert child.returncode == 0
            assert termios.tcgetattr(slave) == before, "terminal not restored"
            print(
                json.dumps(
                    dict(
                        initialize=True,
                        canceled_preview=True,
                        link=True,
                        unlink=True,
                        stale_approval_refused=True,
                        source_preserved=True,
                        terminal_restored=True,
                    )
                )
            )
        finally:
            if child.poll() is None:
                child.kill()
                child.wait()
            os.close(master)
            os.close(slave)


if __name__ == "__main__":
    validate(os.path.abspath(sys.argv[1]))
