# Velnix — Security and Recovery Design

Status: proposed Phase 0 requirements, not implemented guarantees. See [feasibility evidence](FEASIBILITY.md) and proposed ADRs. Local development tools execute user code; same-UID project isolation and production hardening are outside the security promise.

## Trust model

Protect user files, system configuration, runtime integrity, CA private keys, project secrets and local mail. Threats include malicious repositories/configuration, dependency/provider compromise, hostile websites reaching localhost, another local account, unexpected network exposure, concurrent commands and interrupted privileged operations. A compromised root account or the Velnix user's account can bypass these controls; document that limit without weakening cross-user permissions.

Discovery reads bounded metadata and never sources shell profiles, runs framework commands, installs tools, trusts mise configs or loads plugins. Executable probes can themselves execute attacker-controlled code: report untrusted paths statically; require explicit probe/run permission for project-local or otherwise untrusted binaries. Normal known-provider probes use timeouts, bounded output and a controlled environment. Do not follow repository instructions as authorization.

## Privilege boundaries

| Operation | Authority | Boundary and approval |
|---|---|---|
| Registry, runtime resolution, config rendering | Current user | Private XDG state; no sudo |
| User runtime install through a provider | Current user | Separate reviewed install plan; provider/source/version explicit; deferred beyond v0.1 |
| System packages/repositories | One-shot administrator transaction | Exact package/repository plan, dependency/removal preview, no arbitrary shell helper; deferred |
| User workload process | Current user | Owned config, loopback/private sockets, no capabilities or root |
| Existing system service start/stop/reload | Scoped one-shot systemctl/polkit action | Exact known unit/action and separate authority grant; status access grants no lifecycle permission |
| Nginx | Rootless separate instance first | Private prefix and ports 8080/8443; never automatically reload shared system Nginx |
| Linux hosts/trust | Scoped administrator action | Exact owned entry or certificate fingerprint, compare-before-write; manual v0.1 instructions |
| Windows hosts | Windows elevation | Separate preview/manual step; no Linux sudo equivalence |
| Windows trust | Explicit CurrentUser store permission | Exact public CA fingerprint; machine store requires separate elevation |

v0.1 has no installed privileged helper, setuid binary, broad sudoers entry or privileged dashboard. Never sudo the entire Velnix application. Future helper protocols must accept typed operations, strict path/unit allowlists and trusted templates, revalidate target identity immediately before mutation, reject symlinks and user-controlled include paths, and clear loader/PATH environment overrides. Root must never parse arbitrary project-supplied Nginx configuration. Running `nginx -t` as root against user-controlled config is also privileged interpretation, not a safe validation shortcut.

All mutations produce an immutable plan describing inputs, hashes, ownership, exact writes/commands, dependencies, privilege, network use and rollback limits. Dry-run performs no installs, writes, service changes or trust changes; read-only probes disclose their execution. Approval is tied to plan digest and current state. Changed preconditions invalidate approval. Noninteractive commands require explicit plan acceptance; `--yes` must not grant ownership or approve additional privileges. Authorization and cancellation errors have stable CLI/JSON representations.

## Ownership and adoption

Record separately: origin (native package/provider/unknown), management authority (external/created/adopted), and grants (read/configure/lifecycle/remove). A package installed at Velnix's request remains package-manager controlled; provenance is never replaced. Discovery defaults to external/read-only. Adoption is a specific reviewed action with resource identity, current digest, existing references and recovery notes; adoption of a service does not adopt all its packages or data.

Resource records include schema version, stable resource ID, provider ID/version, canonical path or package/unit identity, scope/UID, observed version, created/adopted operation ID, before/after digest, references, grants and last reconciliation status. Track owned config, hosts entries, trust fingerprints and process instances independently. Deduplicate runtime paths by realpath/provider identity without collapsing distinct CLI/FPM capabilities. Velnix never recursively deletes a provider root or project directory. Unlink removes registry/routing references, never source code or databases. Installation intent alone is insufficient removal authority: native packages may acquire external dependents after installation.

## State and interruption

Use user-only config/state directories (0700) and sensitive files (0600); runtime sockets/locks under a validated private XDG_RUNTIME_DIR or a safely created UID-specific fallback, never a predictable world-writable directory. Acquire one per-user mutation lock across CLI/TUI/web; multiple users' independent instances do not share a registry. Validate schema and canonical paths before locks or file access. Resolve path races with descriptor-based operations where available; fail closed on changed ownership or symlinks.

Persist an operation journal before effects. States: planned → approved → applying → verified → complete; failures become failed or recovery-required, never apparent success. Write temp files in the destination filesystem, fsync, atomic rename and directory fsync. Registry commits reference durable resource evidence. After a crash reconcile observed files/processes/provider state against before/after hashes; external transactions may have succeeded before acknowledgment. Never replay package installs or trust changes blindly. Keep bounded journals and redacted backups; preserve recovery evidence while an operation is unresolved.

## Nginx, FPM and app processes

Rootless Nginx uses generated allowlisted directives, separate prefix/PID/log/temp paths and loopback listeners; no arbitrary includes, module loads or config fragments from project files. Validate candidate complete config before replacement and reload, then verify process and HTTP health. [Nginx reload behavior](https://nginx.org/en/docs/beginners_guide.html) retains the old configuration if applying a new one fails; still check application health and record partial failures.

Validate document roots against the approved project root; deny dotfiles, `.env`, VCS/config files, dependency directories as appropriate, and unknown-host requests. Use explicit static, front-controller and generic-PHP profiles; do not assume one rewrite rule supports every framework. Validate SCRIPT_FILENAME and script existence, restrict executable suffixes and forbid writable upload directories as PHP entrypoints. Per-project FPM configs/sockets run as the same user with bounded children. No public FPM port and no permissive socket mode. Same-UID application code can access that user's other files: this tool provides no container-grade isolation.

App launch requires explicit argv, working directory and selected environment. Framework detection never starts package scripts. Forward signals and terminate only verified owned process groups/units. Record Linux boot ID and process start time with PID/executable to avoid PID reuse. Do not enable systemd lingering implicitly. Bind failures preserve the existing listener and report its owner; no automatic service takeover or firewall changes.

## Web and mail (later releases)

Loopback is necessary but insufficient: require a cryptographically random session/bootstrap token, avoid URL tokens in logs/history, strict Host/Origin allowlists, CSRF protection and no permissive CORS. Reject DNS rebinding and browser cross-origin writes; protect WebSocket origins and authenticate reads exposing secrets. Use HttpOnly/SameSite cookies where appropriate, rate/body limits, CSP and no remote asset dependencies. Dashboard cannot elevate; present a plan for terminal approval when privilege is needed. No generic shell endpoint.

SMTP is loopback-only, unauthenticated only within that explicit local threat boundary, with no outbound relay. MIME parsing requires size/depth/time limits. Sandbox HTML previews without scripts, remote images or active links; safe attachment names, forced downloads and quotas. Store mail privately; default finite retention must be chosen before mail ships. Logs and diagnostic exports redact tokens, passwords, message bodies, private keys and environment values.

## Certificates and supply chain

Dedicated local CA per Velnix user; private keys never enter Git, Windows copies, API responses or diagnostic bundles. Trust additions name the exact store and fingerprint. Name constraints in application policy limit issuance to approved local development names; this does not cryptographically constrain a trusted root or make stolen CA keys harmless. CA compromise requires untrusting that root in every affected store and reissuing certificates; removing a key file does not revoke trust. Browser/enterprise policy and cached sessions can delay effects. Do not delete shared or externally created CAs.

Delegate downloads and signature/checksum verification to approved providers, recording source/version/backend. A checksum from the same compromised channel is not independent authenticity. Never pipe remote installers into a privileged shell; adding signing keys/repositories needs its own plan. Pin build dependencies and Go modules, review dependency licenses, generate release checksums/SBOM and run vulnerability analysis before release. Define a private vulnerability-report channel before public release.

## Recovery limits

| Change | Supported compensation | Limit |
|---|---|---|
| Owned config/registry | Restore known prior bytes only if current digest still matches Velnix's last write | Concurrent manual edits require review; no overwriting them |
| Nginx reload | Restore validated config, reload, health-check | Existing connections and transient failures cannot be undone |
| User processes/services | Stop owned instance; restore previously recorded lifecycle state only with approval | Lost requests, jobs and memory state cannot be recovered |
| Package/runtime transaction | Inspect actual provider result; propose repair/removal | Maintainer scripts, migrations, dependency changes, unavailable versions and shared consumers prevent general rollback |
| Hosts entries | Remove only exact owned entries, preserve unrelated lines | Resolver caches and external edits need reconciliation |
| CA trust/certificates | Remove exact owned fingerprint from each approved store | Copies, browser caches and compromise remain; no universal revocation |
| Database data/mail deletion | Restore only from a separately verified backup | No general undo; v0.1 never deletes database data |

On partial failure report completed effects, remaining effects, preserved backups and exact manual recovery steps. Rollback is best-effort compensation with verified preconditions, not an ACID transaction across Linux and Windows.
