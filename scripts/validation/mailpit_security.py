#!/usr/bin/env python3
"""Disposable Linux Mailpit checkpoint; never connects to an existing inbox.

Requires Python 3, an existing Mailpit and, for --trace-egress, strace.
Exit 0: selected checks passed; 1: test failure; 2: strict no-egress gate failed.
This is evaluation code, not a Velnix mail adapter or a complete acceptance suite.
"""
import argparse
import base64
import hashlib
import http.client
import json
import os
from pathlib import Path
import secrets
import signal
import smtplib
import socket
import subprocess
import tempfile
import time
from email.message import EmailMessage


def report(check, **values):
    print(json.dumps(dict(check=check, **values)), flush=True)


def free_port():
    with socket.socket() as listener:
        listener.bind(('127.0.0.1', 0))
        return listener.getsockname()[1]


def run(options):
    executable = Path(options.mailpit)
    if not executable.is_absolute():
        raise ValueError('--mailpit requires an absolute executable path')
    report('executable', path=str(executable), sha256=hashlib.sha256(executable.read_bytes()).hexdigest())
    with tempfile.TemporaryDirectory(prefix='velnix-mailpit-check-') as directory:
        root = Path(directory)
        root.chmod(0o700)
        user, password = 'velnix', secrets.token_hex(32)
        authfile = root / 'auth'
        # SHA1 is used only for the provider's supported htpasswd format, with
        # a random 256-bit password. No reusable user password is involved.
        authfile.write_text(user + ':{SHA}' + base64.b64encode(hashlib.sha1(password.encode()).digest()).decode() + '\n')
        authfile.chmod(0o600)
        smtp, api = free_port(), free_port()
        while smtp == api:
            api = free_port()
        args = [str(executable), '--smtp', f'127.0.0.1:{smtp}', '--listen', f'127.0.0.1:{api}',
                '--allowed-hosts', f'127.0.0.1:{api}', '--ui-auth-file', str(authfile),
                '--database', str(root / 'mail.db'), '--disable-version-check',
                '--smtp-disable-rdns', '--max-message-size', '10', '--max', '500',
                '--max-age', '7d', '--quiet']
        if options.trace_egress:
            args = ['/usr/bin/strace', '-f', '-e', 'trace=connect', '-e',
                    'inject=connect:error=ENETUNREACH', '-o', str(root / 'connect.trace')] + args
        # Deliberately do not inherit MP_*, proxy variables, or host HOME config.
        environment = {'PATH': '/usr/bin:/bin', 'HOME': directory, 'TMPDIR': directory}
        authorization = 'Basic ' + base64.b64encode((user + ':' + password).encode()).decode()

        def request(method, path, authenticated=False, host=None, origin=None, body=None):
            headers = {'Host': host or f'127.0.0.1:{api}'}
            if authenticated:
                headers['Authorization'] = authorization
            if origin:
                headers['Origin'] = origin
            if body is not None:
                headers['Content-Type'] = 'application/json'
            connection = http.client.HTTPConnection('127.0.0.1', api, timeout=5)
            try:
                connection.request(method, path, body, headers)
                response = connection.getresponse()
                data = response.read(2 * 1024 * 1024 + 1)
                assert len(data) <= 2 * 1024 * 1024, 'API response exceeded test limit'
                return response.status, data
            finally:
                connection.close()

        with (root / 'provider.log').open('wb') as log:
            process = subprocess.Popen(args, env=environment, stdout=log, stderr=log,
                                       umask=0o022 if options.normal_umask else 0o077,
                                       start_new_session=True)
            egress_failure = False
            try:
                ready = False
                for _ in range(50):
                    assert process.poll() is None, 'provider exited before ready; flags or port conflict'
                    try:
                        request('GET', '/api/v1/messages')
                        ready = True
                        break
                    except OSError:
                        time.sleep(0.1)
                assert ready, 'provider readiness deadline exceeded'
                message = EmailMessage()
                message['From'] = 'sender@example.test'
                message['To'] = 'recipient@example.test'
                message['Subject'] = 'Phase 6 disposable multipart'
                # The IP link is checked only under strace connect injection.
                message.set_content('plain content http://1.1.1.1/velnix-blocked-spike')
                message.add_alternative('<p>HTML content</p>', subtype='html')
                message.add_attachment(b'\x00\x01\xff', maintype='application', subtype='octet-stream',
                                       filename='../../escape.bin')
                with smtplib.SMTP('127.0.0.1', smtp, timeout=5) as client:
                    client.send_message(message)
                status, body = request('GET', '/api/v1/messages', True)
                listing = json.loads(body)
                assert status == 200 and listing['total'] == 1, 'capture/list failed'
                ident = listing['messages'][0]['ID']
                status, body = request('GET', '/api/v1/message/' + ident, True)
                detail = json.loads(body)
                assert status == 200 and 'plain content' in detail['Text'], 'text preview failed'
                attachment = detail['Attachments'][0]
                assert attachment['FileName'] == '../../escape.bin'
                part = attachment['PartID']
                status, body = request('GET', '/api/v1/message/' + ident + '/part/' + part, True)
                assert status == 200 and body == b'\x00\x01\xff', 'attachment bytes changed'
                assert not (root.parent / 'escape.bin').exists(), 'unexpected automatic attachment export'
                report('multipart capture and binary attachment', passed=True, unsafe_filename_preserved=True)

                with smtplib.SMTP('127.0.0.1', smtp, timeout=5) as client:
                    try:
                        client.sendmail('sender@example.test', ['recipient@example.test'],
                                        b'Subject: oversized\r\n\r\n' + b'x' * (11 * 1024 * 1024))
                    except smtplib.SMTPResponseException as error:
                        assert error.smtp_code == 552, 'unexpected SMTP rejection'
                    else:
                        raise AssertionError('oversized message accepted')
                report('11 MiB message rejected at 10 MiB limit', passed=True)
                nested = b'Content-Type: text/plain\r\n\r\ninner'
                for depth in range(1000):
                    boundary = ('depth' + str(depth)).encode()
                    nested = (b'Content-Type: multipart/mixed; boundary="' + boundary + b'"\r\n\r\n--'
                              + boundary + b'\r\n' + nested + b'\r\n--' + boundary + b'--\r\n')
                started = time.monotonic()
                with smtplib.SMTP('127.0.0.1', smtp, timeout=5) as client:
                    try:
                        client.sendmail('sender@example.test', ['recipient@example.test'], nested)
                    except smtplib.SMTPResponseException:
                        pass
                assert request('GET', '/api/v1/messages', True)[0] == 200
                report('responsive after 1000-level MIME', seconds=round(time.monotonic() - started, 3))

                for label, method, authenticated, host, origin, body, expected in [
                    ('unauthenticated read', 'GET', False, None, None, None, 401),
                    ('authenticated read', 'GET', True, None, None, None, 200),
                    ('hostile Host', 'GET', True, 'evil.example', None, None, 403),
                    ('hostile Origin read', 'GET', True, None, 'https://evil.example', None, 403),
                    ('hostile Origin mutation', 'DELETE', True, None, 'https://evil.example', '{}', 403),
                    ('unauthenticated hostile mutation', 'DELETE', False, None, 'https://evil.example', '{}', 403),
                ]:
                    status, _ = request(method, '/api/v1/messages', authenticated, host, origin, body)
                    assert status == expected, label + ': unexpected HTTP status ' + str(status)
                    report(label, status=status, passed=True)
                modes = {name: oct((root / name).stat().st_mode & 0o777)
                         for name in ('mail.db', 'mail.db-wal', 'mail.db-shm')}
                if not options.normal_umask:
                    assert all(mode == '0o600' for mode in modes.values()), 'unsafe database file modes'
                report('database file modes', umask='022' if options.normal_umask else '077', modes=modes)

                if options.trace_egress:
                    trace = root / 'connect.trace'
                    before = [line for line in trace.read_text().splitlines() if 'connect(' in line]
                    assert not before, 'unexpected connect attempt before explicit link check'
                    status, _ = request('GET', '/api/v1/message/' + ident + '/link-check', True)
                    assert status == 200, 'link-check endpoint unavailable'
                    attempts = [line for line in trace.read_text().splitlines()
                                if 'connect(' in line and '1.1.1.1' in line]
                    egress_failure = bool(attempts)
                    report('strict no-egress gate', passed=not egress_failure,
                           blocked_connect_attempts=len(attempts), traffic_sent=False)
            finally:
                # New session/group belongs only to this disposable test and its
                # provider/strace children, not an existing external instance.
                if process.poll() is None:
                    os.killpg(process.pid, signal.SIGTERM)
                    try:
                        process.wait(timeout=5)
                    except subprocess.TimeoutExpired:
                        os.killpg(process.pid, signal.SIGKILL)
                        process.wait(timeout=5)
                for port in (smtp, api):
                    with socket.socket() as listener:
                        listener.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
                        listener.bind(('127.0.0.1', port))
                report('owned child stopped and ports released', passed=True)
            return 2 if egress_failure else 0


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--mailpit', default='/usr/local/bin/mailpit')
    parser.add_argument('--trace-egress', action='store_true', help='requires strace; block every provider connect syscall')
    parser.add_argument('--normal-umask', action='store_true', help='observe provider database modes under umask 022')
    raise SystemExit(run(parser.parse_args()))
