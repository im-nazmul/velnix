# Phase 1 foundation

This document records the original foundation behavior. Phase 2 now adds project transactions, schema-2 registry support and before/after recovery as documented in [PROJECTS](PROJECTS.md); statements below about initialization being the only mutation describe Phase 1.

Implementation started with the user's instruction “Lets start implementation”. This authorizes Phase 1 foundation, not later roadmap phases. Go/package/state boundaries and scoped unprivileged operations are accepted for this foundation; Nginx, runtime providers, DNS/trust, TUI, web and mail proposals remain pending their milestones.

## Build and use

Go 1.27.3+ on Linux, with a C toolchain for race tests. The local default is Go 1.27.3; production/release builds should use a currently supported patched Go release. CI reads the required toolchain from `go.mod` and runs vulnerability checks with it. Earlier validation records retain the Go version used at the time. No runtime distributions or Go toolchain are bundled.

```sh
go build -trimpath -o bin/velnix ./cmd/velnix
bin/velnix version
bin/velnix doctor
bin/velnix doctor --json
bin/velnix config show
bin/velnix config validate --config config.example.toml
bin/velnix state status --json
bin/velnix state init --dry-run --json
# Review effects and replace PLAN_ID with the exact returned id:
bin/velnix state init --approve PLAN_ID
```

Flags follow the leaf command. Config show emits the effective configuration and file-presence metadata; validate uses the same strict parser. Missing default config uses defaults; an explicitly requested missing file fails. Unknown TOML keys, newer schema, non-regular files, symlinks, files over 64 KiB and out-of-range timeouts are rejected. Config error messages omit supplied values. No config writes or framework/project scanning occur.

Doctor reads Linux release/kernel/PID1/systemd metadata and checks known package-manager paths without invoking them. Package-manager presence grants no ownership or installation permission. User-systemd socket presence is only a hint; systemd/Windows capability is never inferred from distro name alone. WSL mode, Windows DNS, services, runtime versions and browser trust remain unverified. Missing systemd is a warning, not a reason to fail daemonless commands. Invalid config/state or interrupted initialization causes a nonzero result; JSON report stays on stdout and the failure envelope on stderr.

## Private state and consent

XDG config/data/state/cache paths are resolved without creating them. Relative XDG values fall back to HOME defaults; HOME must be absolute for commands using user paths. Version/help do not need HOME. State init is the only ordinary mutation exposed. Preview is the default, and `--dry-run` explicitly selects it. `--approve` must match the current plan digest; stale plans fail. Preview includes the exact data/state paths, before/target digests, effects, authority and recovery limits. No package, service, repository, hosts or trust change is implemented.

Init creates missing data/state ancestors with 0700, an empty schema-1 registry in data, and journal/lock files in state. Existing final Velnix directories must be current-user-owned 0700; files must be regular, current-user-owned 0600 with one hard link. Velnix refuses unsafe state; it does not chmod or adopt existing directories. Directory traversal refuses symlink components (including symlinked home/XDG paths); choose physical XDG paths if needed. Intermediate existing ancestors may have ordinary shared permissions, but private leaves and descriptor-relative operations protect state. Same-user compromise remains outside the security boundary.

The mutation lock lives in the private state directory and works across processes. It remains as an inert file; no daemon or runtime socket is created. OS advisory locks release on process death; no stale PID lock removal is needed. Writes fsync temporary files before atomic publication and fsync parent directories. Initial registry publication uses renameat2(RENAME_NOREPLACE), so even a concurrently introduced destination cannot be overwritten. Filesystems/kernels without that capability fail rather than using an unsafe fallback. One bounded initialization journal is sufficient because Phase 1 has only initialization effects; later operations need reviewed journal retention/migration semantics before project mutation ships.

## Recovery

```sh
bin/velnix state recover --dry-run --json
bin/velnix state recover --approve PLAN_ID
```

An interrupted init with an exact committed target registry can be marked complete. If no registry was committed, recovery marks initialization failed without replaying the registry write; preview init again afterward. Modified/unrecognized registries, missing journal evidence and newer schemas fail closed for manual review. State init is idempotent once complete. No destructive reset command or state-schema migration exists yet: schema 1 is the initial format and other versions are refused. Preserve invalid state for diagnosis rather than deleting it automatically. A pre-journal crash may leave empty private directories/lock files; init can safely be previewed again. After a write failure, inspect status/recover even if the last command returned an error.

## Output and exit codes

| Exit | Meaning |
|---|---|
| 0 | Success (doctor warnings are allowed) |
| 1 | Unexpected I/O or execution failure |
| 2 | Invalid command, config or schema |
| 3 | Unavailable capability or deadline exceeded |
| 4 | Unsafe/inaccessible state permissions or path |
| 5 | Stale plan, concurrent change or recovery conflict |
| 130 | Cancellation |

`--json` writes one result to stdout; errors use `{"error":{"code":"...","message":"..."}}` on stderr. Commands never prompt, making missing approval a safe preview rather than an implicit yes. All application work runs as the invoking user; root is unnecessary.

## Developer checks and limits

```sh
go test -race -count=1 ./...
go vet ./...
go mod verify
go build -trimpath -ldflags='-s -w' -o bin/velnix ./cmd/velnix
```

Tests cover read-only detection, configuration/schema rejection, permissions/symlinks/hard links, plan approval/staleness, concurrent initialization, simulated interrupted journal boundaries, real process lock cancellation and lock release after SIGKILL. These do not prove power-loss durability on every filesystem or full WSL/native compatibility. No automatic migration is necessary until a second supported schema exists. Platform tests use filesystem fixtures, not runtime/provider mocks that could conceal executable side effects.

The CLI uses the standard flag package. TOML parsing delegates to pinned MIT-licensed go-toml/v2 v2.4.3 (which includes upstream nesting limits). Linux descriptor operations and atomic no-overwrite publication use pinned golang.org/x/sys v0.30.0 (BSD-3-Clause), compatible with the local Go baseline. See THIRD-PARTY-NOTICES.md. The module path `velnix` is local until the canonical public repository/name is selected; public import-path migration is a packaging decision, not a plugin ABI. Phase 2 requires separate approval.
