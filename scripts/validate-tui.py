#!/usr/bin/env python3
"""Linux PTY acceptance: python3 scripts/validate-tui.py /absolute/velnix."""
import fcntl
import json
import os
import pty
from pathlib import Path
import select
import signal
import struct
import subprocess
import sys
import tempfile
import termios
import time


def validate(binary, mode):
    with tempfile.TemporaryDirectory(prefix="velnix-tui-") as root:
        master, slave = pty.openpty()
        before = termios.tcgetattr(slave)
        env = dict(os.environ, TERM="xterm-256color", NO_COLOR="1")
        for key in ["CONFIG", "DATA", "STATE", "CACHE"]:
            env[f"XDG_{key}_HOME"] = f"{root}/{key.lower()}"
        if mode == "config-error":
            config_dir = Path(env["XDG_CONFIG_HOME"]) / "velnix"
            config_dir.mkdir(parents=True)
            (config_dir / "config.toml").write_text("[invalid")

        def snapshot():
            return {str(path.relative_to(root)): (path.stat().st_mode,
                    None if path.is_dir() else path.read_bytes())
                    for path in Path(root).rglob("*")}

        before_files = snapshot()
        fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 100, 0, 0))
        process = subprocess.Popen([binary, "tui"], stdin=slave, stdout=slave,
                                   stderr=slave, env=env)
        output = bytearray()

        def drain(duration):
            end = time.monotonic() + duration
            while time.monotonic() < end:
                if select.select([master], [], [], .05)[0]:
                    try:
                        output.extend(os.read(master, 65536))
                    except OSError:
                        break

        try:
            drain(1)
            if mode == "config-error":
                os.write(master, b"4")
                drain(.3)
                os.write(master, b"\x1b")
                drain(.2)
            for key in [b"\x10", b"projects", b"\r", b"h"]:
                os.write(master, key)
                drain(.2)
            for key in [b"l", b"l", b"l", b"l", b"h", b"?", b"\x1b", b"r",
                        b"\r", b"j", b"\x1b", b"/", b"HTTP", b"\r"]:
                os.write(master, key)
                drain(.15)
            fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 8, 24, 0, 0))
            process.send_signal(signal.SIGWINCH)
            drain(.3)
            os.write(master, b"\x1b")
            drain(.1)
            if mode == "signal":
                process.send_signal(signal.SIGTERM)
            else:
                os.write(master, b"\x03" if mode == "ctrl-c" else b"q")
            drain(1)
            process.wait(timeout=5)
            assert process.returncode == (130 if mode == "signal" else 0)
            assert termios.tcgetattr(slave) == before, "terminal modes not restored"
            assert snapshot() == before_files, "query changed state"
            if mode == "config-error":
                assert b"Environment needs a look" in output
                assert b"Configuration" in output
            assert b"\x1b[?1049l" in output, "alternate screen not restored"
            for name in [b"v e l n i x", b"What would you like to do?", b"Overview", b"Projects", b"Runtimes", b"Services",
                         b"Diagnostics", b"Registered projects", b"Provider capabilities",
                         b"System databases", b"Not configured", b"Filter: HTTP"]:
                assert name in output, f"missing screen/detail: {name!r}"
            print(json.dumps(dict(mode=mode, exit=process.returncode,
                                  terminal_restored=True, state_unchanged=True)))
        finally:
            if process.poll() is None:
                process.kill()
                process.wait()
            os.close(master)
            os.close(slave)


if __name__ == "__main__":
    for exit_mode in ["quit", "ctrl-c", "signal", "config-error"]:
        validate(os.path.abspath(sys.argv[1]), exit_mode)
