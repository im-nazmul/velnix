# Phase 4: multi-language

Authorized by the user: “Lets start work with phase 4, no need to ask for my
aproval. you can start implementation directly”. This starts Phase 4 without
marking Phase 3 platform acceptance complete. Phase 3 FPM/HTTPS implementation
is now recorded in [web validation](validation/PHASE-3-WEB.md); real mkcert and
platform/trust acceptance remain open.

The first coherent slice adds read-only Node, Bun and Python candidate discovery
through the application service and CLI. No new dependency or state schema is
needed. Existing PHP list output and selection/exec behavior remain compatible.

```sh
velnix runtime list --kind node --json
velnix runtime list --kind bun --directory "$HOME/.bun/bin" --json
velnix runtime list --kind python --directory /absolute/project/.venv/bin --json
```

By default only `/usr/bin` and `/usr/local/bin` are scanned. `--directory`
replaces those defaults with one explicit absolute clean directory. Discovery
does not scan PATH, shell profiles, runtime-manager stores or project trees.
Directory symlinks and executable aliases are resolved; canonical executable
paths are deduplicated and sorted. Only executable regular-file candidates with
exact names `node`, `nodejs`, `bun`, `python` or numeric Python suffixes are listed.
No version is inferred from a filename. An empty candidate list means no matching
candidate was found in the scanned directories, not that the runtime is absent
from the machine. Directory read failures appear in `issues`, including an
explicit permission-denied reason when applicable. Broken aliases and
non-executable entries are ignored.

Candidates have unknown origin, external authority and `probed: false`. Discovery
does not run even a version command, so names cannot establish runtime identity;
scripts and manager shims can appear as candidates. No installation, adoption,
selection, state write, dependency installation or system mutation occurs.
`--directory` is currently limited to the three new kinds; the default PHP list
keeps its existing contract.

## Exact runtime inspection

Inspection now extends the PHP preview/approve convention to Node, Bun and
Python. Supply an absolute native executable and its exact installed version,
without Node's leading `v` or Python's `Python ` prefix:

```sh
velnix runtime inspect --kind node --executable /absolute/node --version 25.9.0 --json
velnix runtime inspect --kind bun --executable /absolute/bun --version 1.4.3 --dry-run --json
velnix runtime inspect --kind python --executable /usr/bin/python3 --version 3.12.3 --json
velnix runtime inspect --kind python --executable /usr/bin/python3 --version 3.12.3 --approve PLAN_ID --json
```

The example versions are local validation observations, not required versions;
use the actual version you intend to inspect. Preview hashes the executable and
prints its canonical path, identity, argv, environment, cwd and plan ID without
running it. Repeat the same request with that ID to authorize the probe. Changing
kind, version or executable identity invalidates approval. `--dry-run` and
`--approve` cannot be combined. Successful inspection reports `probed: true`
with unknown origin and external authority; it persists nothing.

Only native ELF binaries of at most 128 MiB without group/world-write or set-ID
bits are accepted; scripts and manager shims are refused. Node and Bun receive
`--version`; Python receives `-I -S --version`. Probes use cwd `/`, no stdin,
a fixed minimal environment, a two-second timeout and 4 KiB output limits per
stream. Nonzero exits, stderr and mismatched version output fail. Version output
does not prove executable safety or language identity. The operation runs the
explicitly approved binary as the current user and provides no sandbox.

Python aliases resolve to their canonical interpreter; inspection does not
validate virtualenv semantics. Persistent selection/exec for these three kinds
remains unavailable. PHP commands and persisted selection format are retained.
See [ADR 0010](decisions/0010-multi-language-inspection.md) and
[local validation](validation/PHASE-4-RUNTIME.md).

Remaining Phase 4 work: exact runtime selection/exec with
language-specific environment rules, nonmutating provider resolution, explicit
loopback app-server proxying and bounded framework detection. No app-server
scripts are automatically launched. Provider mutations and later roadmap phases
remain outside this slice.

Tests cover nonexecution of executable shims, symlink deduplication, filename
filtering, non-executable files, directories, broken links, invalid requests,
missing-directory reporting and CLI output without state creation. This is local
implementation validation, not WSL/native Ubuntu platform acceptance.
