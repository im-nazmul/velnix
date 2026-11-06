#!/usr/bin/env python3
"""Validate the built local UI with disposable state; print no credentials."""

import argparse
import http.client
import json
import os
import select
import signal
import socket
import subprocess
import tempfile
import time
from http.cookies import SimpleCookie
from pathlib import Path


def main(binary):
    binary = str(Path(binary).resolve())
    with tempfile.TemporaryDirectory(prefix="velnix-ui-check-") as directory:
        root = Path(directory)
        root.chmod(0o700)
        runtime = root / "runtime"
        runtime.mkdir(mode=0o700)
        environment = dict(
            os.environ,
            HOME=directory,
            XDG_CONFIG_HOME=str(root / "config"),
            XDG_DATA_HOME=str(root / "data"),
            XDG_STATE_HOME=str(root / "state"),
            XDG_CACHE_HOME=str(root / "cache"),
            XDG_RUNTIME_DIR=str(runtime),
        )
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            port = listener.getsockname()[1]
        arguments = ["ui", "--port", str(port)]

        def command(args, expected=0):
            result = subprocess.run(
                [binary] + args + ["--json"],
                env=environment,
                capture_output=True,
                text=True,
                timeout=10,
            )
            assert result.returncode == expected, (result.returncode, result.stderr)
            return json.loads(result.stdout) if result.stdout else None

        plan = command(arguments + ["--dry-run"])
        assert sorted(p.name for p in root.iterdir()) == ["runtime"], "UI preview created state"
        command(arguments + ["--approve", "incorrect"], 5)
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", port))
            listener.listen()
            command(arguments + ["--approve", plan["id"]], 5)
        assert not list(runtime.iterdir()), "occupied port created credentials"
        started = time.monotonic()
        process = subprocess.Popen(
            [binary] + arguments + ["--approve", plan["id"], "--json"],
            env=environment,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        try:
            assert select.select([process.stdout], [], [], 5)[0], "UI startup timed out"
            ready = json.loads(process.stdout.readline())
            startup_ms = round((time.monotonic() - started) * 1000, 2)
            secret_file = Path(ready["bootstrap_file"])
            assert secret_file.stat().st_mode & 0o777 == 0o600
            assert secret_file.parent.stat().st_mode & 0o777 == 0o700
            secret = secret_file.read_text().strip()
            origin = ready["address"]

            def request(method, path, body=None, cookie=None, csrf=None, override=None):
                headers = {"Host": f"127.0.0.1:{port}"}
                if body is not None:
                    headers.update({"Content-Type": "application/json", "Origin": origin})
                if cookie:
                    headers["Cookie"] = cookie
                if csrf:
                    headers["X-CSRF-Token"] = csrf
                if override:
                    headers.update(override)
                connection = http.client.HTTPConnection("127.0.0.1", port, timeout=5)
                try:
                    connection.request(
                        method, path, None if body is None else json.dumps(body), headers
                    )
                    response = connection.getresponse()
                    data = response.read(2 * 1024 * 1024 + 1)
                    return response.status, dict(response.getheaders()), data
                finally:
                    connection.close()

            assert request("GET", "/api/read/overview")[0] == 401
            assert request("GET", "/api/challenge", override={"Host": "evil.example"})[0] == 403
            assert (
                request("GET", "/api/challenge", override={"Origin": "https://evil.example"})[0]
                == 403
            )
            status, headers, body = request("GET", "/api/challenge")
            assert status == 200
            anonymous = SimpleCookie()
            anonymous.load(headers["Set-Cookie"])
            anonymous_cookie = "velnix_login=" + anonymous["velnix_login"].value
            nonce = json.loads(body)["csrf"]
            status, headers, body = request(
                "POST", "/api/login", {"token": secret}, anonymous_cookie, nonce
            )
            assert status == 200 and secret.encode() not in body
            assert not secret_file.exists(), "bootstrap remains after consumption"
            cookies = SimpleCookie()
            cookies.load(headers["Set-Cookie"])
            session = "velnix_session=" + cookies["velnix_session"].value
            csrf = json.loads(body)["csrf"]
            for page in (
                "overview",
                "projects",
                "runtimes",
                "services",
                "certificates",
                "mail",
                "diagnostics",
            ):
                assert request("GET", "/api/read/" + page, cookie=session)[0] == 200, page
            assert (
                request("POST", "/api/plan", {"operation": "initialize"}, session, "wrong")[0]
                == 403
            )
            assert sorted(p.name for p in root.iterdir()) == ["runtime"], (
                "passive dashboard initialized state"
            )
            fields = Path(f"/proc/{process.pid}/status").read_text().splitlines()
            rss_kib = int(next(line.split()[1] for line in fields if line.startswith("VmRSS:")))
            assert rss_kib <= 60 * 1024, f"UI RSS exceeds 60 MiB target: {rss_kib} KiB"
            assert request("POST", "/api/logout", {}, session, csrf)[0] == 200
            assert request("GET", "/api/session", cookie=session)[0] == 401
            process.send_signal(signal.SIGINT)
            stdout, stderr = process.communicate(timeout=8)
            assert process.returncode == 130, stderr
            assert not list(runtime.iterdir()), "owned runtime directory survived normal shutdown"
            with socket.socket() as listener:
                listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                listener.bind(("127.0.0.1", port))
            print(
                json.dumps(
                    {
                        "dashboard_cli": "passed",
                        "warm_launch_ms": startup_ms,
                        "rss_after_passive_reads_kib": rss_kib,
                        "checks": [
                            "read-only launch plan",
                            "incorrect approval",
                            "occupied port",
                            "private bootstrap",
                            "Host/Origin rejection",
                            "authenticated reads",
                            "CSRF rejection",
                            "passive state unchanged",
                            "logout revocation",
                            "SIGINT cleanup",
                            "port released",
                        ],
                    }
                )
            )
        finally:
            if process.poll() is None:
                process.kill()
            process.communicate(timeout=8)


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary")
    main(parser.parse_args().binary)
