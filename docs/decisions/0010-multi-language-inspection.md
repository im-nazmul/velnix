# ADR 0010 — Exact existing multi-language runtime inspection

- Status: Accepted within authorized Phase 4
- Date: 2026-10-20
- Authorization: “Lets start work with phase 4, no need to ask for my aproval. you can start implementation directly”

## Decision

Extend the existing PHP preview/approve pattern to Node, Bun and Python
inspection. Extract the descriptor-based executable fingerprint and launch
mechanism into `internal/providers/nativeexec`; preserve the existing PHP
selection JSON representation through a domain type alias. There is no registry
schema migration or new dependency. PHP retains its existing language-specific
probe and project selection behavior.

Require an explicit absolute executable path and exact patch version. Resolve
aliases, require a regular native ELF executable at most 128 MiB, refuse
group/world-write and set-ID bits, and hash bytes plus device/inode identity.
Preview does not execute anything. The approval digest binds kind, executable
identity, required version, argv, cwd, fixed environment and disclosed limits.
Changed preconditions invalidate approval. Approved inspection creates no state
or management grants, returning external authority and unknown provenance.

Use only `--version` for Node and Bun and `-I -S --version` for Python. Execute
the checked descriptor through Linux procfs, with canonical path as argv[0]. Use
cwd `/`, no stdin, only `PATH=/usr/bin:/bin` and `LANG=C`, a two-second timeout
and 4 KiB retained output per stream. Require exact version output (allowing a
trailing newline), successful exit and empty stderr. Recheck identity after the
probe. A version response is not proof of language identity or executable safety.

## Limits and consequences

Native binaries may execute arbitrary same-UID code once approved. This is not
a sandbox and does not pin libraries or all machine configuration. Scripts and
manager shims are unsupported. Python 2 and versions without isolated mode are
unsupported by this probe. Inspection of a virtualenv executable alias reports
its canonical interpreter identity, not a validated virtualenv execution spec.
No package-manager, framework or app-server commands are run.

Persistent multi-language project selection and execution remain separate work;
in particular Python virtualenv paths must not be collapsed into an executable
identity and mistaken for an environment. Non-PHP selection/clear/resolve fails
explicitly in the application service. Installation, provider hooks, automatic
downloads and global defaults remain outside this slice.

## Command references

- [Bun version check](https://bun.com/docs/installation)
- [Python command-line isolated and site options](https://docs.python.org/3/using/cmdline.html)
