# Phase 2 HTTP completion proposal

Status: approved by the user with “You have my approval” on 2026-10-20. Implemented scope and recovery limits are documented in [HTTP](HTTP.md).

This completes the project/HTTP milestone without entering Phase 3. The registry
already supports reviewed link/unlink transactions. The remaining implementation
will use the existing application/state boundaries and mutation lock.

## Proposed decisions

- Accept ADR 0003 only for a separate, unprivileged static HTTP Nginx instance.
  Bind IPv4 loopback on configurable unprivileged ports, default 8080. Use the
  installed executable explicitly; never install packages or touch system Nginx.
  HTTPS, FPM and port 8443 remain Phase 3 work.
- Accept ADR 0006 only for foreground Nginx, with explicit start, status, stop
  and reload commands. No detached supervisor, systemd integration or lingering.
  A separate terminal may request stop; exiting/canceling the foreground command
  cleans up its owned workload.
- Use generated complete configuration in a private owned prefix. Project names
  map to their registered NAME.test hosts; unknown hosts are rejected. Expose
  static files only, disable directory listings, deny sensitive and executable
  source paths, and prevent symlink escapes. Do not interpret project config.
- Start/reload/stop follow read-only plan and exact approval semantics. Plans
  bind registry/configuration and executable/directory/process identities.
  Record PID, boot ID, process start time and executable identity; use a Linux
  process handle where available to avoid signaling a reused PID. Fail closed
  when ownership cannot be established.
- Validate candidate configuration before publication. Verify the owned process
  and HTTP response after start/reload. Preserve external edits and occupied
  listeners. Journal effects and report recovery-required when a partial effect
  cannot be safely reconciled; never silently replay a process launch.
- Keep access logging disabled by default and bound retained diagnostics. The
  foreground owner drains child output and performs cleanup on cancellation.
- Link/unlink while HTTP is active require an explicit reviewed reload to apply
  the new routing snapshot. Report pending routing changes clearly; document
  that unlink alone does not revoke an already active route. Stop HTTP first
  when immediate revocation is required.

## Implementation and exit evidence

1. Add focused Nginx configuration, validation and process adapters, application
   orchestration, then CLI commands; keep policy outside CLI handlers.
2. Test hostile paths/hosts, sensitive file denial, symlinks, plan invalidation,
   process identity mismatch, concurrent mutations and interrupted effects.
3. Exercise real installed Nginx in disposable private test state: serve two
   linked projects, reject unknown hosts, preserve an occupied listener, reject
   invalid candidates without replacing working config, reload routing, and
   stop/cancel without leaving owned workers behind. Compare external config
   before/after. Fake executables alone do not satisfy these checks.
4. Run the full Go suite with race detection, vet and diff checks; document
   command usage, manual hosts setup, cleanup and recovery limitations.
5. Record actual local results separately from unavailable native Ubuntu or
   Windows/WSL networking evidence. Do not claim unperformed matrix checks pass.

Approval of this proposal authorizes implementation and disposable local HTTP
tests, not host package/service/DNS/trust changes. Each shipped mutation still
requires its own reviewed CLI plan. Phase 3 remains outside this authorization.
