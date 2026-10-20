# Velnix — Phase 0 Feasibility

Research date: 2026-10-20. Status: proposal for approval; no application or host changes performed. Documentation research establishes plausible designs, not tested platform compatibility. ADRs remain proposed. The original principles in AGENTS.md remain authoritative.

## Specification review

| Finding | Consequence | Proposed resolution |
|---|---|---|
| FEATURES says four runtimes “initially”, while ROADMAP spreads them across phases | Initial release has no boundary | v0.1 acceptance scope in IMPLEMENTATION-PLAN; catalogue describes the eventual product |
| PROJECT says native Linux later; AGENTS requires portability | WSL assumptions could enter core | Native Ubuntu acceptance from v0.1; WSL-specific code only in adapters |
| Runtime ownership mixes package authority and Velnix intent | Installing a package could incorrectly authorize removal | Separate provenance, management grants and transaction evidence; SECURITY defines each |
| Global/project version selection lacks execution semantics | Shell PHP may differ from FPM | Child-process execution, independent CLI/FPM resolution; no update-alternatives or shell edits |
| Nginx “isolated config” lacks a privilege boundary | Root Nginx could interpret user-written directives | Separate unprivileged instance first; shared system instance deferred |
| `.test` and HTTPS are requirements without resolver/trust contracts | Windows browser success could be falsely claimed | Per-host entries and separate Windows/Linux/browser checks; no wildcard DNS in v0.1 |
| CLI tree includes broad install/update/remove without authorization semantics | Scripts could silently modify machines | Reviewed operation plans; v0.1 runtime mutation deferred |
| State format, concurrency and interruption are unspecified | CLI/web can overwrite state or lose partial changes | Versioned JSON registry, interprocess lock, durable operation journal |
| Mail/web breadth lacks lifecycle and retention requirements | Unbounded storage and unwanted listeners | Optional foreground processes; bounded retention; later releases |
| No release license, support policy or performance baseline | Public release and support claims are premature | Choose license before publication; explicit matrix and unmeasured budgets |

Additional requirements: schema migrations with backup and refusal of newer schemas; symlink/path validation; deterministic version conflicts; IPv4/IPv6 behavior; multiple WSL distributions and port collision diagnostics; cancellation/timeouts; accessibility for later interfaces; redacted diagnostic export; offline behavior; supply-chain verification; no automatic dependency scripts. Project code is trusted only after an explicit run action, never merely because a directory was linked. Native execution is not a sandbox.

## Runtime provider comparison

Providers are optional dependencies, never bundled distributions. Detect only installed tools; never install a manager just to inspect it. Exact provider versions and supported machine-readable outputs must be pinned in adapter compatibility tests before support is claimed.

| Provider | Useful capabilities | Costs / limits | Recommendation |
|---|---|---|---|
| Native packages | Signed repository integration, system services, PHP CLI/FPM packages | Root transactions, distro-specific versions, maintainer scripts, shared dependencies; not an exact-version manager | Read-only discovery first; explicit package plans later |
| mise | Multi-language resolution and install delegation, Node/Bun/Python support, child-process execution | Backends have different trust/build behavior; project config can contain executable features; exec can auto-install | Preferred optional Node/Bun adapter after isolation/auto-install controls are verified; no blanket trust |
| uv | Python discovery, managed interpreters, `.python-version`, project virtual environments | Can automatically download Python; uses Astral python-build-standalone distributions; venv packages and interpreter ownership differ | Preferred optional Python adapter; prohibit implicit downloads and system environment mutations |
| nvm | Existing per-user Node installations, `.nvmrc`, `which` and `exec` | Shell function, shell startup assumptions; Node only | Respect existing installs; read-only exact executable resolution first, constrained integration later |
| phpenv + php-build | Side-by-side PHP builds, version selection, configurable FPM | Compilation/toolchain and extension ABI burden; FPM availability depends on build | Detect externally owned installations; no build automation in v0.1 |
| PHPBrew | PHP variants/extensions and FPM commands | PHP bootstrap, source builds, host build dependencies and maintenance | Optional future specialist adapter; not default |
| pyenv | Existing Python versions and shims | Source builds and platform dependencies | Read-only discovery; use real interpreter and venv paths |
| Bun upstream installer | Standalone runtime and version installation | Installer/profile changes and platform CPU/libc constraints | Respect existing Bun; delegate explicit future installation, prefer mise where already used |

Sources: [mise Node](https://mise.jdx.dev/lang/node.html), [Bun](https://mise.jdx.dev/lang/bun.html), [backends](https://mise.jdx.dev/dev-tools/backends/), [exec and automatic installation](https://mise.jdx.dev/cli/exec.html), [security](https://mise.jdx.dev/security.html); [uv interpreter provenance and discovery](https://docs.astral.sh/uv/concepts/python-versions/), [uv virtual environments](https://docs.astral.sh/uv/pip/environments/); [nvm upstream](https://github.com/nvm-sh/nvm); [phpenv](https://github.com/phpenv/phpenv), [php-build](https://github.com/php-build/php-build), [PHPBrew](https://github.com/phpbrew/phpbrew); [Bun installation](https://bun.sh/docs/installation). PHP support through mise must be evaluated by named backend: the official core-language pages do not establish a supported FPM contract. Do not promise that mise installs a usable PHP-FPM pair.

### Resolution and execution

Explicit override > Velnix project requirement > recognized version files > Velnix user default > discovered system default. Requirements are not installed versions: resolve to an exact installation and report source, provider and path. Conflicting files at the same tier require a choice; do not use directory iteration order. Recognize only documented literal formats initially (`.nvmrc`, `.node-version`, `.python-version`, `.tool-versions`); do not evaluate templates, hooks or shell expressions. Respect `requires-python` as a constraint and report incompatibility with the selected interpreter. A missing requested version is an error, never a fallback to another major version or an installation trigger. Project config is explicit opt-in and cannot grant privileges.

A proposed `velnix exec -- command args` selects an absolute executable and child PATH without changing the parent shell. Composer must run under the selected PHP; npm/pnpm/yarn must resolve within the selected Node toolchain. Package dependency installation remains a separate explicit action. Existing virtual environments take precedence for Python execution only when their interpreter satisfies the chosen requirement; stale or incompatible environments fail clearly. Global defaults affect Velnix commands only. Later provider-specific writes require a preview of the exact file changes.

### PHP CLI and FPM feasibility

CLI and FPM are separate capabilities. Record binary realpath, version, SAPI, ini paths, extension set and package/provider identity independently. CLI extension success says nothing about FPM extensions. Version-specific Unix sockets route each project to its selected FPM installation; never switch all projects by changing a global socket or `/usr/bin/php`.

Proposed v0.1: existing versioned FPM binaries launched as the current user, with Velnix-owned per-project configuration, socket, PID and log paths. A project pool uses `pm=ondemand` and a bounded child count. The non-root master cannot switch users: omit privileged user switching and validate actual upstream behavior in a disposable spike. Deny network FPM listeners; use a private runtime directory and restrictive socket permissions. Pool separation prevents accidental routing and configuration coupling, not isolation between projects sharing a UID. Verify actual HTTP PHP version as well as CLI output. Shared system pools remain external and read-only.

Ubuntu repositories do not guarantee arbitrary simultaneous PHP minor versions. Never add a third-party repository silently; explain repository signing, maintenance and removal limits if a later installation feature needs one. Exact patch pins may become unavailable. Extensions must match ABI/SAPI; no automatic PECL builds. Sources: [PHP FPM configuration](https://www.php.net/manual/en/install.fpm.configuration.php), [FPM network warning](https://www.php.net/manual/en/install.fpm.php).

## WSL2, DNS and HTTPS

[Microsoft systemd guidance](https://learn.microsoft.com/en-us/windows/wsl/systemd) requires WSL 0.67.6+ for systemd support, describes opt-in `/etc/wsl.conf`, and warns that services do not keep a WSL instance alive. Probe PID 1 and manager availability rather than infer them from distro name. User systemd can also be unavailable even when system systemd works. No automatic enablement, lingering, distro restart or WSL shutdown.

[Microsoft networking guidance](https://learn.microsoft.com/en-us/windows/wsl/networking) documents Windows-to-WSL localhost access under NAT, optional mirrored mode on supported Windows 11, DNS tunneling and firewall considerations. DNS tunneling carries Linux DNS queries through Windows; it does not publish Linux `.test` zones to Windows. Loopback access must be tested on the actual host, especially with VPNs, firewall policies, multiple distributions, IPv6 and port conflicts. Do not bind all interfaces to fix connectivity. Do not hardcode WSL IPs or create portproxy/firewall rules automatically.

`.test` is reserved for testing by [RFC 2606](https://www.rfc-editor.org/rfc/rfc2606.html); reservation does not create DNS answers. v0.1 proposes exact hostnames in Linux and Windows hosts files, each separately approved/manual. Hosts files do not provide wildcards. Map to IPv4 loopback initially; publish AAAA/IPv6 hosts entries only after listener and Windows forwarding checks pass. High ports remain in URLs. Native Linux needs only its own resolver configuration. Alias entries have independent lifecycle. Wildcard DNS through dnsmasq/systemd-resolved and Windows DNS/NRPT is deferred: privileged resolver changes, VPN interactions, port 53 and cross-host forwarding make it a separate feature.

Use external mkcert with a dedicated Velnix CAROOT for optional certificates; do not implement new certificate cryptography. Separate CA creation, issuance, Linux trust installation and Windows trust installation. [mkcert upstream](https://github.com/FiloSottile/mkcert) supports local trust workflows and explicitly warns about CA key compromise. Copy only the public CA certificate to Windows. Prefer Windows CurrentUser Root with explicit consent; LocalMachine trust is a separate administrative action. [Import-Certificate](https://learn.microsoft.com/en-us/powershell/module/pki/import-certificate) documents store targeting. Linux trust and browser NSS stores are separate from Windows trust; browser policy can prevent trust. Check hostname/SAN, expiry and actual browser access; do not equate curl success with browser success. Manual Windows hosts/trust instructions are acceptable v0.1, with unverified status until Windows checks pass.

## Execution model and feasibility verdict

A daemonless control plane is feasible. CLI/TUI invoke application services directly; web invokes the same services while its optional process runs. Nginx, FPM, databases and requested app servers are workload processes, not a Velnix control daemon. Prefer foreground lifetimes and optional user-systemd units; do not build a generic supervisor or require experimental provider daemon features. A detached process requires recorded executable/start identity, bounded logs and explicit stop semantics. No PID-only killing.

Mail requires an actual SMTP listener while capture is active. Evaluate an existing local mail tool such as [Mailpit](https://github.com/axllent/mailpit) before implementing SMTP/MIME/storage. Bind SMTP and viewer to loopback, disable relay and remote exposure, cap message/attachment bytes and retention, sanitize hostile previews. Mail and web are excluded from v0.1; future evaluation must verify flags against the selected release.

Main feasibility gates: prove rootless Nginx/FPM under a private prefix; prove two independently selected PHP CLI/FPM versions; verify provider discovery never installs or executes repository hooks; verify Windows `.test` HTTPS on NAT and mirrored hosts. These are planned approval-dependent experiments, not completed tests. If a gate fails, revise scope/ADR before adding privileged workarounds.
