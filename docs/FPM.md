# Per-project PHP-FPM

Phase 3 adds existing native FPM pools, independently from [CLI PHP](RUNTIMES.md).
Run as an ordinary user with installed FPM and Linux pidfd support. Velnix owns
only generated configuration, lifecycle evidence and private sockets. Existing
system FPM services, packages and project sources remain external.

Initialize and link a project with a dedicated public document root, then review:

```sh
velnix fpm start --name demo --php-fpm /usr/sbin/php-fpm8.3 --version 8.3.35 --profile front-controller --uploads uploads,files --dry-run --json
velnix fpm start --name demo --php-fpm /usr/sbin/php-fpm8.3 --version 8.3.35 --profile front-controller --uploads uploads,files --approve PLAN_ID --json
```

Use the actual installed exact version. Preview runs no executable. Approval
binds the executable SHA-256/device/inode, project identities, profile, upload
paths, socket path, generated configuration and prior evidence. The approved
command verifies `-n -v` reports the exact version and `fpm-fcgi` SAPI, validates
the generated configuration, then runs a foreground owner until canceled/stopped.
The owner returns its final result on exit; inspect status in another terminal.

```sh
velnix fpm status --name demo --json
velnix http start --dry-run --json
velnix http start --approve HTTP_PLAN_ID --json
```

Start pools before starting/reloading HTTP. HTTP plans bind each running pool's
record and socket identity. Missing/stopped pools refuse new routing plans;
`http status` reports a `routing_error` and pending routing. Stopping a pool
leaves active Nginx routing in place until explicitly reloaded/stopped; requests
to that unavailable backend fail with 502. FPM lifecycle never launches Nginx.

Each pool has one static worker and a private 0600 Unix socket under 0700 XDG
state. A long state path can exceed the supported 100-byte socket path limit;
use a shorter private XDG state location. No TCP FPM listener, systemd service,
privileged user switch or shared service configuration is created. The chosen
native executable is pinned through a descriptor for probe/validation/launch.

FPM runs with a minimal environment and installed default PHP ini/extensions;
PHPRC, loader settings and other inherited overrides are absent. Those installed
configuration files, modules and shared libraries are not identity-pinned. Review
changes to them separately. Application requests execute user code and can load
project `.user.ini` through PHP's normal behavior. No application command or
framework hook runs automatically at startup. Workers use the project directory;
projects sharing a UID are not security sandboxes. Diagnostics are drained with
an 8 KiB retained tail; no unbounded private disk log is configured.

The routing profiles are:

- Static: projects without an enabled pool keep existing static-only routing.
- `php`: existing lowercase `.php` files are passed to the selected pool; `/`
  prefers `index.php`, then `index.html`. Other executable source is denied.
- `front-controller`: only `/index.php` executes directly; non-file application
  routes fall back to it with the original URI/query string. Other PHP paths
  remain denied. Static `index.html` is still eligible when present.

Nginx checks file existence and disables symlink traversal before passing scripts.
There is no PATH_INFO splitting or arbitrary project Nginx/FPM fragment. Dotfiles,
sensitive directories, source suffixes and built-in `uploads` are denied before
PHP dispatch. `--uploads` defaults to `uploads`; additional comma-separated clean
relative paths are also denied entirely, including static files. List every
application upload directory. A dedicated public root remains essential: the
rules cannot infer every secret filename or upload directory. FPM restricts main
scripts to `.php`, disables CGI path-info guessing and imposes a 30-second request
termination limit. This does not prevent application code reading other files.

## Stop, disable and recovery

```sh
velnix fpm stop --name demo --dry-run --json
velnix fpm stop --name demo --approve STOP_PLAN_ID --json
velnix fpm disable --name demo --dry-run --json
velnix fpm disable --name demo --approve DISABLE_PLAN_ID --json
velnix http reload --dry-run --json
velnix http reload --approve RELOAD_PLAN_ID --json
```

Stop interrupts requests and preserves the enabled PHP choice. Disable requires
a stopped/recovered pool and removes that choice for future HTTP routing plans.
Explicit HTTP reload applies static routing. To change FPM version/profile, stop,
review a fresh start, then review HTTP reload. CLI PHP selection is unchanged.
Stop pools before unlinking a project; unlink does not stop existing workloads.

Ctrl-C/SIGTERM in the owner terminates its verified master/workers. A killed
master triggers cleanup of recorded workers and leaves recovery evidence. Signals
use boot ID, start time, executable identity and pidfds, never name/PID alone.
An owned leftover socket is removed only with its recorded device/inode. Existing
or replaced sockets are preserved. No automatic restart occurs.

```sh
velnix fpm recover --name demo --dry-run --json
velnix fpm recover --name demo --approve RECOVERY_PLAN_ID --json
```

`fpm-NAME-http.json`, `fpm-NAME-http.conf` and before/candidate files use the
same transition engine and shared mutation lock as HTTP. Recovery stops verified
owned processes and marks stopped; it never replays a launch. External edits,
unknown socket identities and the launch-intent window before process identity
persistence require manual review. Preserve all evidence rather than guessing
from a PID file. Files may remain after stop; no recursive cleanup is performed.

[Validation](validation/PHASE-3-WEB.md) records local results and open acceptance.
The configuration follows the [PHP FPM manual](https://www.php.net/manual/en/install.fpm.configuration.php)
and [Nginx FastCGI contract](https://nginx.org/en/docs/http/ngx_http_fastcgi_module.html).
