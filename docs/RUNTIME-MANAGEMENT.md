# Velnix — Runtime Management

Implemented Phase 3: [existing PHP CLI selection and exec](RUNTIMES.md),
[independent FPM pools](FPM.md) and [optional HTTPS](HTTPS.md).
The precedence, configuration and broader provider capabilities below describe
future direction; they are not implemented by the current direct selection path.

## Goal
Unified control without reinventing installers or bundling language runtimes.

## Ownership classes
1. System managed: native package manager (APT/DNF/Pacman).
2. External manager: mise/nvm/uv/pyenv/phpenv etc.
3. Velnix managed: installation explicitly initiated and tracked by Velnix through a provider.
4. Unknown: discoverable but not safe to mutate automatically.

## Provider capabilities
Detect/list/inspect; optionally install/update/uninstall/set-default/set-project-version/resolve. Report unsupported operations explicitly.

## Proposed version precedence
Explicit command override > Velnix project config > recognized project version files > user global default > system default. Report conflicts.

## Example provisional project config
```toml
[project]
name = "example"
domain = "example.test"
type = "laravel"

[runtimes]
php = "8.5"
node = "24"

[web]
driver = "nginx"
document_root = "public"

[https]
enabled = true
```

## PHP
Distinguish CLI PHP from PHP-FPM; investigate multiple versions, pools/sockets, extension availability, Composer compatibility and per-project FastCGI routing. Shell PHP changes must not be mistaken for web PHP changes.

## Node/Bun
Investigate native packages, mise, nvm, version files, package managers and dev-server lifecycle.

## Python
Respect system Python, uv/pyenv, project virtual environments, interpreter selection and ASGI/WSGI servers. Never modify system Python unsafely.

## Dependencies
Respect Composer, npm/pnpm/yarn/Bun and Python environment conventions. Per-project runtime version does not imply dependency isolation.

## Safe removal
Verify provider and ownership, check project references, preview changes, require confirmation. Do not promise complete rollback of native package transactions.

## Research
Compare mise and specialized providers; executable resolution, deduplication, PHP-FPM routing, safe upgrades and migration paths.

## Phase 0 clarification

The ownership classes above describe installation provenance; they do not grant management or deletion authority. Track provenance and explicit management grants separately as defined in [SECURITY](SECURITY.md). The proposed v0.1 supports existing installations and child-process selection, with independent PHP CLI/FPM resolution; install/update/remove and provider defaults are later capabilities. See [FEASIBILITY](FEASIBILITY.md) for the provider comparison, deterministic conflict behavior and Python environment rules.
