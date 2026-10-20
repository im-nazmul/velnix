# ADR 0012 — Existing multi-language selections and external app proxies

- Status: Implemented within authorized Phase 4
- Date: 2026-10-20
- Authorization: “Now lets finish phase 4”, following the Phase 4 implementation authorization

Reuse project registry transactions with schema 4 for independent Node, Bun and
Python selections and an explicit IPv4 loopback upstream port. Retain schema-3
PHP data and earlier recovery rules. Older readers refuse newer schemas;
subsequent PHP changes must not downgrade the registry. Relink retains every
setting; unlink/clear never delete external installations or project data.

The native provider resolves only reviewed exact external executable identities.
Passive resolve checks bytes/identities, literal requirements and venv metadata;
it executes nothing. Optional mise/uv adapters have no established compatible
no-download/no-hook contract in this environment and remain explicitly unavailable
through capability reporting. Existing externally managed native binaries work by
absolute path. Shell activation, global defaults and provider mutations are
outside this implementation, consistent with the approved fallback contract.

Use existing isolated probes before registry commits. Bind supported raw metadata
fingerprints into selection plans and recheck metadata before commit. Explicit
exec pins the executable descriptor, passes literal argv, uses project cwd,
sanitizes inherited language/loader variables and supplies deterministic child
PATH plus VELNIX entrypoint variables. Bun defaults to no implicit installation.
User code is trusted only on explicit exec and is not sandboxed or supervised
beyond the direct child. PHP's existing exec environment contract is retained.

Record Python venv identity separately from its underlying interpreter; collapse
neither into the other. Support real project-contained directories, literal
pyvenv.cfg with exact matching home/version and no system site packages, and a
verified bin/python alias. Preserve launcher argv[0] during pinned execution and
verify real sys.prefix behavior. Selection/resolve never activate scripts or
initialize site packages. Changed/new/incompatible environments require review.

Read a fixed root manifest/version-file set with no symlink traversal, a 64 KiB
per-file cap and nonblocking regular-file checks. Frameworks are hints only.
Require all supported literal requirements to agree; implement a documented
numeric comparator subset, failing explicitly on unsupported syntax. This is
neither a universal dependency resolver nor a complete semver/PEP 440 parser.

Reverse proxying targets only an explicitly configured already-running
127.0.0.1 unprivileged port. No server launch or process ownership is inferred.
Reuse reviewed Nginx start/reload, candidate validation, TLS and recovery. After
approval, TCP readiness is checked before publishing routing; application identity
is not inferred from a listener. Preserve unknown-host and sensitive-path denies,
forward original requests and overwrite forwarding claims. Support HTTP/1.1
upgrade tunnels and refuse direct self-proxy loops. FPM and proxy conflict.
Metadata changes require explicit HTTP reload; clearing metadata is not immediate
route revocation. App servers remain external even after HTTP stops.

[Phase 4 usage](../PHASE-4.md) and [validation](../validation/PHASE-4-EXECUTION.md)
record implementation limits and open provider/platform acceptance.

Reference contracts: [Python venv](https://docs.python.org/3.12/library/venv.html),
[Node environment options](https://nodejs.org/api/cli.html),
[Bun auto-install](https://bun.com/docs/runtime/auto-install),
[Nginx proxy](https://nginx.org/en/docs/http/ngx_http_proxy_module.html) and
[WebSocket forwarding](https://nginx.org/en/docs/http/websocket.html).
