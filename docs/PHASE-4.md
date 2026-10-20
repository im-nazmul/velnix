# Phase 4: multi-language

**Status: COMPLETE — approved implementation scope, 2026-10-20.**
[Completion evidence](validation/PHASE-4-COMPLETION.md) records the final provider
adapters and checks. [Platform/release acceptance](validation/RELEASE-ACCEPTANCE.md)
remains separately open; implementation completion does not assert those passes.

Authorized by “Lets start work with phase 4, no need to ask for my aproval. you
can start implementation directly”, followed by “Now lets finish phase 4”. The
bounded implementation now includes existing Node/Bun/Python discovery,
inspection, persistent selection/exec, native and mise/uv store resolution, explicit
loopback app-server proxying and passive framework detection. Local validation
is in [PHASE-4-EXECUTION](validation/PHASE-4-EXECUTION.md).

Phase 3 implementation remains intact. Its real-mkcert, platform and trust
acceptance is still open in [PHASE-3-WEB](validation/PHASE-3-WEB.md). This task
adds no runtime installer, manager activation, global default, app-server
supervisor, TUI, mail or dashboard.

## Existing executable discovery and inspection

```sh
velnix runtime list --kind node --json
velnix runtime list --kind bun --directory /absolute/existing/bin --json
velnix runtime list --kind python --directory /absolute/project/.venv/bin --json
velnix runtime inspect --kind node --executable /absolute/node --version 25.9.0 --dry-run --json
velnix runtime inspect --kind node --executable /absolute/node --version 25.9.0 --approve INSPECT_PLAN_ID --json
velnix runtime providers --json
```

Use the actual exact installed version. Discovery reads only `/usr/bin` and
`/usr/local/bin` by default; one explicit `--directory` replaces them for these
three kinds. It does not scan PATH, manager stores, shell profiles or project
trees. Exact executable candidate names, alias deduplication, directory issues
and unprobed external/unknown provenance retain their existing contract.

Inspection accepts native ELF binaries up to 128 MiB without group/world-write
or set-ID bits, fingerprints path/device/inode/content and previews without
execution. Approval authorizes a two-second isolated version query with cwd `/`,
no stdin, minimal environment and 4 KiB output limits per stream. Node/Bun use
`--version`; Python uses `-I -S --version`. Exact output and empty stderr are
required. Scripts and manager shims are refused; a version response does not
prove safety or language identity. Inspection persists nothing.

`runtime providers` reports capabilities, not ownership/management grants. The
native provider supports exact existing installations by native executable path.
Mise and uv also have read-only `existing-store-v1` adapters, described below.
Resolve checks the selected executable and project metadata without running it.
No provider command, shell activation, download, hook or installation is needed.

## Existing mise and uv stores

```sh
velnix runtime locate --provider mise --provider-root /absolute/mise-data --kind node --version 25.9.0 --json
velnix runtime select --provider mise --provider-root /absolute/mise-data --kind node --version 25.9.0 --name demo --dry-run --json
velnix runtime select --provider mise --provider-root /absolute/mise-data --kind node --version 25.9.0 --name demo --approve PLAN_ID --json
velnix runtime locate --provider uv --provider-root /absolute/uv-python --kind python --version 3.12.3 --json
velnix runtime select --provider uv --provider-root /absolute/uv-python --kind python --version 3.12.3 --name demo --dry-run --json
# Review and repeat the same uv selection with --approve PLAN_ID.
```

Supply the actual installed numeric patch version. `locate` is passive and creates
no state. It reports expected version and executable fingerprint with `probed:
false`, external authority and unknown origin. Names/layouts cannot establish
version, safety or installer provenance. Selection includes the provider source
in its reviewed plan and requires the existing approved version probe before
commit. `--provider` cannot combine with `--executable`/`--php`; the root is always
explicit, and a missing version is an error with no installation attempt.

Mise's root is its **data directory**, containing `installs/node/VERSION/bin/node`
or `installs/bun/VERSION/bin/bun`. Exact version links can resolve into an identity
store inside that data directory. Custom install stores outside the supplied root
are refused; use a direct executable selection for them. No project mise config,
plugins, aliases such as latest, tasks or hooks are read/executed.

Uv's root is its **Python installation directory**, containing
`cpython-VERSION-linux-x86_64-gnu/bin/pythonMAJOR.MINOR` or the corresponding
`aarch64` key. Inspection reads at most 1024 directory entries. Musl, foreign
architectures, free-threaded/PyPy/other layouts are explicitly unsupported by this
adapter; use a supported native direct path instead. Directory links and binary
aliases must remain within the explicit canonical storage root. A root resolving
to `/` is refused. There is no recursive store scan or implicit environment lookup.

The adapters pin this storage contract rather than a manager executable version:
neither manager is ever launched. Tests use documented-layout fixtures and
copies of real installed runtimes, not installed mise/uv releases. Command-driven
manager resolution, installation and activation are outside Phase 4. nvm/pyenv
shell activation stays unavailable; existing native installations still work by
direct path. Persisted selections remain ordinary exact native/venv selections,
so subsequent resolve/exec needs no manager or new schema.

## Persistent selection and execution

Initialize state and link the project first. PHP's existing `--php` commands and
PHP-default `exec` remain compatible. For each additional language:

```sh
velnix runtime select --name demo --kind node --executable /absolute/node --version 25.9.0 --dry-run --json
velnix runtime select --name demo --kind node --executable /absolute/node --version 25.9.0 --approve SELECT_PLAN_ID --json
velnix runtime resolve --name demo --kind node --json
velnix exec --name demo --kind node -- server.js
velnix exec --name demo --kind bun -- run server.ts
velnix exec --name demo --kind python -- -m my_application
velnix runtime clear --name demo --kind node --dry-run --json
velnix runtime clear --name demo --kind node --approve CLEAR_PLAN_ID --json
```

Selection approval authorizes the isolated exact-version query and registry
transaction together. Plans bind executable and venv identities, raw metadata
fingerprints and prior state. Failed probes commit no selection. Resolve/exec
recheck the executable, directory identities, requirements and virtualenv;
missing/stale/incompatible selections fail with no PATH fallback or download.
Relink retains selections. Clear/unlink remove metadata, preserve external files,
and do not stop an explicit command already running.

The first new persisted setting upgrades the registry to schema 4 through the
existing journal/lock/atomic replacement flow. PHP selections, resources and other
projects are retained. Later PHP edits do not downgrade it. Older readers refuse
schema 4; do not downgrade an active installation. `state recover` recognizes
exact before/after states and never replays a probe or application command.

Explicit `exec` passes literal argv without a shell, uses the registered project
root, connects stdin/stdout/stderr and returns child exit status. Cancellation
forwards the terminal signal or SIGTERM and waits up to two seconds before
killing/reaping its direct child. Detached descendants are not supervised.

Node/Bun/Python child environments inherit ordinary application variables but
remove inherited `NODE_*`, `BUN_*`, `PYTHON*`, `LD_*`, `DYLD_*`, `VELNIX_*` and
stale virtualenv activation variables. `resolve` reports unset prefixes/variables
and the replacement environment delta. PATH is the selected binary directory
(or venv bin) followed by `/usr/bin:/bin`, rather than inherited manager shims.
Node/Bun require a matching `node`/`bun` command in that directory. Parent PATH
and environment are unchanged. `VELNIX_NODE`, `VELNIX_BUN` and `VELNIX_PYTHON`
identify the selected entrypoint. Base Python does not invent a `python` alias;
nested commands should use `VELNIX_PYTHON` (or `sys.executable`). Bun selection
does not implicitly select a Node runtime for external commands.

Bun execution adds `--no-install` before user arguments, disabling default
implicit module installation, even with a project requesting forced auto-install.
This is not a sandbox: an explicit user command/script can run package-manager
commands, make network requests, execute hooks or change files. Dependency
installation and application code are never triggered by link/detect/resolve.

## Python virtual environments

An existing project `.venv` is considered during Python selection; a custom venv
requires `--venv /absolute/project/environment`. No venv is created or repaired.
It must be a real directory inside the linked project with a real `bin` directory,
a `bin/python` alias resolving to the selected native interpreter, and a bounded
regular `pyvenv.cfg`. Its literal home and exact version must match that interpreter
and `include-system-site-packages` must be `false`. Metadata hash and directory
identity are stored separately from the canonical interpreter.

Resolve is passive: no activation script, pip, site initialization or repository
hook is evaluated. Compatible venv execution preserves the launcher argv[0],
sets `VIRTUAL_ENV`/child PATH and uses the pinned underlying descriptor; real
Python tests verify `sys.prefix`/`sys.base_prefix`. Once explicit execution begins,
Python's normal site/packages/application behavior applies. Appeared, changed,
missing, aliased or incompatible venvs require a fresh reviewed selection; they
never silently replace an existing base interpreter selection. Exotic virtualenv
layouts, system-site-packages and manager shims are unsupported.

## Requirements and framework hints

```sh
velnix projects detect --name demo --json
```

Detection only reads these files at the registered root: `.nvmrc`,
`.node-version`, `.bun-version`, `.python-version`, `.tool-versions`,
`package.json`, `composer.json` and `pyproject.toml`. Each file is limited to
64 KiB, must be regular, and cannot traverse symlinks; FIFOs/devices/oversized or
malformed supported metadata fail visibly. No recursive scan, scripts, imports,
build backends, framework CLI or package manager runs. Results contain evidence,
a raw metadata digest, literal requirements and dependency-based framework hints.
Hints cover Angular, React, Vue, Vite, Next.js, Nuxt, Express, Fastify, Laravel,
Symfony, Django, Flask and FastAPI; they do not establish that an app boots or
that its route/public directory should change.

Version files require a single exact patch version (Node may have leading `v`);
`.tool-versions` supports exact `nodejs`/`node`, `bun` and `python` entries. Aliases
such as `lts/*`, executable expressions and multiple versions are refused.
Requirements also include package `engines.node`/`engines.bun`, PEP 621
`project.requires-python`, and literal Poetry `dependencies.python`.

Every supported requirement for the selected kind must agree. Exact file
conflicts are errors. Supported manifest syntax is a bounded numeric release
subset: exact/partial versions, `*`/`x`, adjacent `>=`, `<=`, `>`, `<`, `=`, `==`,
`!=` comparators, comma/space AND, `^`, `~`, `~=`, and equality/exclusion suffix
wildcards (`3.12.*`). `~=` requires at least two components. OR, hyphen ranges,
markers and general prerelease/range semantics are unavailable and fail visibly.
This is not a full npm-semver or PEP 440 implementation. Nothing installs a
missing requested version or infers a global default.

## Loopback application proxy

Start the server yourself using an explicit command. Configure only its numeric
unprivileged IPv4 loopback port:

```sh
velnix projects proxy --name demo --port 3000 --dry-run --json
velnix projects proxy --name demo --port 3000 --approve PROXY_PLAN_ID --json
velnix http start --dry-run --json
velnix http start --approve HTTP_PLAN_ID --json
# For an existing Nginx instance, preview/approve http reload instead.
```

Metadata approval does not connect to or start a server. HTTP start/reload binds
the registry/configuration and, only after approval, verifies a TCP listener
exists at `127.0.0.1:PORT`. It does not prove application identity or health.
Missing upstreams fail before configuration publication. Requests to a backend
that later exits return 502; no replacement/restart is attempted. The app server
remains external and is never signaled or owned by Velnix.

Only registered exact domains are proxied. Static-file handling is replaced for
that project; existing sensitive-path denies still apply before proxying. The
original URI/query/method/body are forwarded, with Host and connection-derived
forwarding headers. Client-supplied forwarding claims are overwritten. HTTP/1.1
upgrade forwarding supports WebSocket/HMR tunnels. No external hostname/URL,
credentials, path fragments, arbitrary config, redirects, Unix socket or remote
IP can be configured. Self-proxy to Velnix's selected HTTP/HTTPS listener is
refused. Ports must be 1024–65535. Configure only an app you intend to expose
through its exact `.test` name; choosing a listener does not establish its owner.

Stop/disable FPM before configuring a proxy; active/enabled FPM and proxy routing
conflict visibly. CLI runtime selections remain independent. HTTP metadata edits
require explicit reload/stop to affect active routes. To return to static mode,
review `projects proxy --name demo --port 0`, then review HTTP reload. Disabling
proxy metadata alone does not revoke an active route. Optional [HTTPS](HTTPS.md)
uses the same proxy and supplies the external `https` forwarding scheme.

See [ADR 0012](decisions/0012-phase4-execution-and-proxy.md) and
[ADR 0013](decisions/0013-existing-provider-store-resolution.md) for boundaries,
[completion validation](validation/PHASE-4-COMPLETION.md) for final evidence and
[release acceptance](validation/RELEASE-ACCEPTANCE.md) for the remaining platform gates.
