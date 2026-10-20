#!/usr/bin/env python3
"""Measure foundation commands without changing real user state or flushing caches."""
import argparse
import gzip
import json
import math
import os
from pathlib import Path
import platform
import statistics
import subprocess
import tempfile
import time

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--binary', default='bin/velnix')
parser.add_argument('--output', required=True)
args = parser.parse_args()
binary = Path(args.binary).resolve()

def summary(values):
    ordered = sorted(values)
    return {'p50_ms': round(statistics.median(values), 3),
            'p95_ms': round(ordered[math.ceil(len(ordered) * .95) - 1], 3),
            'samples_ms': [round(v, 3) for v in values]}

with tempfile.TemporaryDirectory(prefix='velnix-benchmark-') as scratch:
    scratch = Path(scratch)
    env = dict(os.environ, HOME=str(scratch),
               XDG_CONFIG_HOME=str(scratch / 'config'),
               XDG_DATA_HOME=str(scratch / 'data'),
               XDG_STATE_HOME=str(scratch / 'state'),
               XDG_CACHE_HOME=str(scratch / 'cache'))
    commands = {'version': ['version', '--json'], 'help': ['--help'],
                'doctor': ['doctor', '--json'],
                'config': ['config', 'show', '--json'],
                'init-plan': ['state', 'init', '--dry-run', '--json']}
    results = {}
    for name, command in commands.items():
        times = []
        for i in range(60):
            start = time.perf_counter_ns()
            subprocess.run([str(binary), *command], env=env, check=True,
                           stdout=subprocess.DEVNULL, stderr=subprocess.PIPE)
            times.append((time.perf_counter_ns() - start) / 1_000_000)
        results[name] = {'first_10_fresh_processes': summary(times[:10]),
                         'warm_filesystem_50_fresh_processes': summary(times[10:])}
    if list(scratch.iterdir()):
        raise RuntimeError('Read-only commands wrote to their isolated HOME/XDG roots')
    doctor = json.loads(subprocess.check_output([str(binary), 'doctor', '--json'], env=env))
    rss = None
    if Path('/usr/bin/time').is_file():
        measured = subprocess.run(['/usr/bin/time', '-f', '%M', str(binary), 'doctor', '--json'],
                                  env=env, check=True, stdout=subprocess.DEVNULL,
                                  stderr=subprocess.PIPE, text=True)
        rss = int(measured.stderr.strip())
    report = {'schema': 1, 'toolchain': subprocess.check_output(['go', 'version'], text=True).strip(),
              'kernel': platform.release(), 'machine': platform.machine(),
              'logical_cpus_visible': os.cpu_count(),
              'filesystem': subprocess.check_output(['stat', '-f', '-c', '%T', str(scratch)], text=True).strip(),
              'cache_note': 'No cache flush. Every sample is a fresh process; first 10 are NOT cold-cache measurements.',
              'host_scope': 'Current execution environment only; not full Windows/native compatibility validation.',
              'binary_bytes': binary.stat().st_size,
              'gzip_binary_bytes': len(gzip.compress(binary.read_bytes(), mtime=0)),
              'doctor_peak_rss_kib': rss,
              'platform_facts': doctor['platform'], 'results': results}
Path(args.output).write_text(json.dumps(report, indent=2) + '\n')
for name, result in results.items():
    print(name, result['warm_filesystem_50_fresh_processes']['p95_ms'], 'ms p95')
