# Phase 4: multi-language

Authorized by the user: “Lets start work with phase 4, no need to ask for my
aproval. you can start implementation directly”. This starts Phase 4 without
marking the remaining Phase 3 FPM/HTTPS work or platform acceptance complete.

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

Remaining Phase 4 work: approved exact runtime inspection/selection/exec with
language-specific environment rules, nonmutating provider resolution, explicit
loopback app-server proxying and bounded framework detection. No app-server
scripts are automatically launched. Provider mutations and later roadmap phases
remain outside this slice.

Tests cover nonexecution of executable shims, symlink deduplication, filename
filtering, non-executable files, directories, broken links, invalid requests,
missing-directory reporting and CLI output without state creation. This is local
implementation validation, not WSL/native Ubuntu platform acceptance.
