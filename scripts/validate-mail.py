#!/usr/bin/env python3
"""Exercise the built Velnix CLI with disposable mail state and existing Mailpit."""
import argparse
import json
import os
from pathlib import Path
import signal
import smtplib
import socket
import subprocess
import tempfile
import time


def free_port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return sock.getsockname()[1]


def main(binary, mailpit):
    binary = str(Path(binary).resolve())
    mailpit = str(Path(mailpit).resolve())
    with tempfile.TemporaryDirectory(prefix='velnix-mail-cli-') as directory:
        root = Path(directory)
        root.chmod(0o700)
        runtime = root / 'runtime'
        runtime.mkdir(mode=0o700)
        env = dict(os.environ, HOME=directory, XDG_DATA_HOME=str(root/'data'),
                   XDG_STATE_HOME=str(root/'state'), XDG_CONFIG_HOME=str(root/'config'),
                   XDG_CACHE_HOME=str(root/'cache'), XDG_RUNTIME_DIR=str(runtime))
        smtp, api = free_port(), free_port()
        while smtp == api:
            api = free_port()
        serve = ['mail','serve','--mailpit',mailpit,'--smtp-port',str(smtp),'--api-port',str(api)]
        processes = []

        def call(args, expected=0):
            result = subprocess.run([binary]+args+['--json'],env=env,capture_output=True,text=True,timeout=15)
            assert result.returncode == expected, (args, result.returncode, result.stderr)
            return json.loads(result.stdout) if result.stdout else None

        def apply(args):
            plan = call(args)
            return call(args+['--approve',plan['id']])

        def start():
            plan = call(serve)
            process = subprocess.Popen([binary]+serve+['--approve',plan['id'],'--json'],env=env,
                                       stdout=subprocess.PIPE,stderr=subprocess.PIPE,text=True)
            processes.append(process)
            # Poll status with a deadline rather than blocking on stdout.readline.
            for _ in range(80):
                assert process.poll() is None, 'CLI mail owner exited before ready'
                status = call(['mail','status'])
                if status['available']:
                    return process, status['record']
                time.sleep(0.1)
            raise AssertionError('CLI mail readiness deadline exceeded')

        try:
            assert not call(['mail','status'])['available']
            assert not (root/'state').exists(), 'read created state'
            apply(['state','init'])
            owner, record = start()
            with smtplib.SMTP('127.0.0.1',smtp,timeout=5) as client:
                client.sendmail('sender@example.test',['receiver@example.test'],
                                'From: sender@example.test\r\nTo: receiver@example.test\r\nSubject: CLI acceptance\r\n\r\nPersistent plain text')
            listing = call(['mail','list'])
            assert listing['total'] == 1
            ident = listing['messages'][0]['ID']
            message = call(['mail','show',ident])
            assert 'Persistent plain text' in message['text']
            assert not call(['mail','list'])['messages'][0]['Read'], 'show mutated read status'
            owner.send_signal(signal.SIGINT)
            stdout,stderr = owner.communicate(timeout=8)
            assert owner.returncode == 130, (owner.returncode,stderr)
            assert 'mail_ready' in stdout
            assert not Path(record['session'],'credentials.json').exists()
            assert not call(['mail','status'])['available']
            call(['mail','list'],expected=3)
            owner, record = start()
            assert call(['mail','list'])['total'] == 1
            # Crash the Velnix owner, not the provider. Linux Pdeathsig must stop
            # this exact provider child without a persistent supervisor.
            owner.kill()
            owner.communicate(timeout=5)
            for _ in range(50):
                status = call(['mail','status'])
                if not status['alive']:
                    break
                time.sleep(0.1)
            assert not status['alive'] and status['recovery_required']
            apply(['mail','recover'])
            assert not call(['mail','status'])['recovery_required']
            assert not Path(record['session'],'credentials.json').exists()
            for port in (smtp,api):
                with socket.socket() as sock:
                    sock.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1)
                    sock.bind(('127.0.0.1',port))
            print(json.dumps({'mail_cli':'passed','checks':['passive status','SMTP capture','plain-text show',
                  'no read-status mutation','SIGINT cleanup','restart persistence','owner-crash child cleanup',
                  'reviewed recovery','credential removal','ports released']}))
        finally:
            for process in processes:
                if process.poll() is None:
                    process.kill()
                process.communicate(timeout=8)


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('binary')
    parser.add_argument('--mailpit',default='/usr/local/bin/mailpit')
    options = parser.parse_args()
    main(options.binary,options.mailpit)
