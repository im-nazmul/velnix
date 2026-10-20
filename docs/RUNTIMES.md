# Existing PHP CLI runtimes

The first Phase 3 slice supports native installed PHP CLI binaries. It never
installs runtimes, changes shell defaults or takes ownership of installations.
[FPM](FPM.md) and optional [HTTPS](HTTPS.md) are implemented separately;
selecting CLI PHP does not change web serving.

## Discover and inspect

```sh
velnix runtime list --json
velnix runtime inspect --php /usr/bin/php8.3 --version 8.3.12 --json
velnix runtime inspect --php /usr/bin/php8.3 --version 8.3.12 --approve PLAN_ID --json
```

Use the exact version actually installed. Discovery reads only standard-directory
metadata, deduplicates canonical paths and reports candidates as unprobed and
external, with unknown origin. It cannot establish that a candidate is PHP.
Inspection previews the executable identity, checksum and isolated probe argv;
only matching approval executes it. Any absolute native binary path may be
supplied, including externally managed installations. Shell shims, relative
paths, set-ID or group/world-writable binaries are refused.

The probe has a two-second timeout, no stdin, cwd `/`, a fixed minimal environment,
`-n` to disable ini loading, and 4 KiB output limits per stream. It verifies exact
PHP_VERSION and CLI SAPI. It does not inspect application extensions or ini; those
belong to the explicit user command environment.

## Select for a linked project

After initializing state and linking the project:

```sh
velnix runtime select --name demo --php /usr/bin/php8.3 --version 8.3.12 --json
velnix runtime select --name demo --php /usr/bin/php8.3 --version 8.3.12 --approve PLAN_ID --json
velnix runtime resolve --name demo --json
```

Selection approval authorizes the isolated probe and registry update together;
a previous standalone inspection is optional. Preview runs no executable and
creates no state. A failed probe commits nothing. Selection stores path,
device/inode, SHA-256 and exact version in schema 3. Existing projects/resources
are retained; older binaries cannot read the upgraded registry. Resolve executes
nothing, returning the checked executable, project directory and child environment
delta. Replacing/updating PHP invalidates selection; review a fresh selection.
Missing selections never silently fall back to PATH or another PHP version.

## Execute

```sh
velnix exec --name demo -- -v
velnix exec --name demo -- artisan about
velnix exec --name demo -- -r 'echo PHP_VERSION, " ", PHP_SAPI, PHP_EOL;'
```

Everything after the required `--` is passed directly to PHP as literal argv.
The command runs in the linked project root with connected stdin and raw child
output. PHP ini, extensions and application environment are inherited normally.
The child receives `VELNIX_PHP` with the absolute selected executable and `PWD`
with the project root. Parent environment and PATH are unchanged. Nested commands
must use `VELNIX_PHP` or PHP_BINARY to select the same PHP; a bare `php` resolves
through inherited PATH. Composer scripts and repository hooks are never run
automatically; an explicit application command can run user code of its choice.

Exec has no `--json` option. It returns the child's exit code (or 128 plus signal
number), with no extra diagnostic for child failure. These codes can overlap
Velnix's diagnostic codes; launch/selection errors have a `velnix:` diagnostic,
whereas child output remains verbatim. Terminal SIGINT/SIGTERM is forwarded;
context cancellation gives the direct child two seconds before kill/reap and
returns Velnix's cancellation/deadline diagnostic. No background supervisor is
created; independently detached descendants are not managed.

## Clear and recover

```sh
velnix runtime clear --name demo --json
velnix runtime clear --name demo --approve PLAN_ID --json
velnix state recover --json
velnix state recover --approve PLAN_ID --json
```

Clear removes only CLI selection metadata. Unlink also drops that project's
selection; neither stops an already running command or deletes executables or
source. Relinking identical project metadata preserves selection. Interrupted
registry changes use the same exact before/after recovery as project linking;
unknown edits require manual review and probes are never replayed by recovery.

All effective registry changes invalidate the HTTP routing snapshot digest,
including runtime metadata changes. Existing static routes continue until an
explicit reviewed HTTP reload or stop; CLI selection does not enable PHP over
HTTP. See [HTTP](HTTP.md) for lifecycle/recovery behavior and
[validation](validation/PHASE-3-RUNTIME.md) for performed checks and open gates.

## Node, Bun and Python discovery

Phase 4 adds `runtime list --kind node|bun|python` with optional absolute
`--directory`. These are unprobed candidates. Explicit version inspection is
available through `runtime inspect --kind node|bun|python --executable /absolute/binary
--version X.Y.Z`, with preview and matching `--approve PLAN_ID` required before
execution. Persistent selection, resolution and explicit application execution now use
the same transaction model with independent per-language settings.
See [PHASE-4](PHASE-4.md) for commands and limits.
