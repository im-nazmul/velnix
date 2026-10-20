# Phase 1 local validation

Scope: foundation only, using the user's system Go toolchain. No temporary toolchain was installed. This is a local validation record, not full v0.1 acceptance or a security audit.

## Environment

- Toolchain: go version go1.22.2 linux/amd64
- Platform: Ubuntu 24.04, Linux amd64, WSL kernel 6.18.54.1-microsoft-standard-WSL2.
- Execution namespace PID 1: `codex`; system systemd unavailable. Diagnostics handled this as a warning.
- Visible logical CPUs: 12; temporary filesystem reported by stat: ext2/ext3.
- Reference hardware (4 cores/8 GiB/ext4) was not reproduced; no Windows build/browser/network-mode assertions made.

## Completed checks

- `go test -race -count=1 ./...` — passed.
- `go vet ./...` — passed.
- `go mod verify` — all modules verified.
- Release-style `go build -trimpath -ldflags='-s -w' -o bin/velnix ./cmd/velnix` — passed.
- Compiled CLI smoke check using temporary HOME/XDG roots: preview creates nothing, explicit approval initializes private state, status validates the journal, repeated init has no effects, stale approval is rejected, doctor reports healthy with platform warnings.
- Live read-only doctor and config validation — passed without initializing actual user state.
- Tests exercise malformed/oversized/deeply nested TOML and FIFO/symlink rejection; registry schema/JSON errors; private directory/file permissions; symlink/hardlink refusal; no-replace initial publication; concurrent initialization; stale approval; simulated interrupted journal stages; interprocess lock cancellation and real SIGKILL lock release.

Simulated journal boundary cases do not prove full transaction fault injection or power-loss durability. Disk-full, hardware failure and every filesystem-specific failure mode remain future integration work. No package/service/network/trust mutation was attempted.

## Preliminary performance

Reproduction:

```sh
python3 scripts/benchmark-foundation.py --output /tmp/velnix-foundation-samples.json
```

[Raw samples and environment metadata](phase-1-local-samples.json): 10 initial fresh-process samples, followed by 50 fresh-process samples with warm filesystem caches. No caches flushed, no other process sampling or CPU/memory limits imposed. Python subprocess overhead is included. First samples are explicitly not cold-cache measurements. Commands use absent, temporary HOME/XDG state; the harness asserts that read-only commands created no user files.

| Command | Warm p50 | Warm p95 |
|---|---|---|
| `version` | 1.331 ms | 1.921 ms |
| `help` | 1.138 ms | 1.777 ms |
| `doctor` | 1.487 ms | 1.875 ms |
| `config` | 1.221 ms | 1.648 ms |
| `init-plan` | 1.333 ms | 1.697 ms |

Stripped binary: 2400408 bytes; gzip-compressed binary: 1046034 bytes (not a packaged release archive). GNU time reported doctor peak 2864 of 2864 KiB. Provider discovery does not execute subprocesses, and this milestone has no Velnix listeners or workload processes. Results meet the preliminary help/version and doctor budgets on this host; they do not validate project-list/resolve/workload budgets that have no implementation yet.

## Pending external validation

GitHub Actions configuration is added with commit-pinned actions, Go 1.22.2 compatibility and Go 1.27.3 release-toolchain jobs, plus govulncheck v1.9.0 on the release toolchain. Hosted CI and vulnerability scanning have not been run locally. The local Go 1.22.2 build establishes compatibility, not release-toolchain security validation. Native Ubuntu, full Windows NAT/mirrored/browser/trust scenarios, power-loss/filesystem fault injection and supported release-toolchain measurements remain pending. Later roadmap milestones require separate approval.
