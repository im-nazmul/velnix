# ADR 0013 — Read-only mise and uv installation-store adapters

- Status: Implemented; completes the remaining Phase 4 implementation item
- Date: 2026-10-20
- Authorization: user requested finishing remaining Phase 4 work and marking complete

Extend the existing native provider with explicit, bounded filesystem adapters
for installed mise Node/Bun and uv CPython. This completes nonmutating provider
resolution without executing a manager command. No new installation capability,
network behavior, ownership grant, runtime dependency or registry schema is added.

Pin our supported storage contract as `existing-store-v1`, rather than claiming
compatibility with arbitrary manager versions or their machine-readable output.
No manager executable/output protocol is used. The existing command-adapter
compatibility restriction in ADR 0012 still applies to future command invocation;
it does not prevent reading a documented explicit installation layout.

Require an explicit canonicalized storage root and exact numeric patch version.
Mise uses DATA/installs/node|bun/VERSION/bin/node|bun and may follow exact-version
links into an identity store inside DATA. Reject executable paths outside DATA,
including custom external install stores; those use direct executable selection.
Uv supports the exact CPython Linux amd64/arm64 GNU directory key and its
bin/pythonMAJOR.MINOR entry. Limit uv directory inspection to 1024 entries.
No recursive discovery, host defaults, project manager configuration, aliases
such as latest, automatic downloads, activation, manager hooks or shell sourcing.
Unsupported layouts fail with direct-path remediation rather than guessed paths.

A passive `runtime locate` reports executable identity, storage layout, expected
version, external authority, unknown origin and `probed: false`. A directory name
does not establish runtime version or installer provenance. Provider-backed
selection resolves the same candidate and includes adapter/root/entry/executable
evidence in the reviewed plan. The existing approved exact-version probe must
succeed before registry commit. Recheck the source under the mutation lock;
changed targets invalidate approval. Keep persisting the native executable/venv
selection so resolve/exec never need to launch a manager or depend on one remaining
installed. No schema upgrade beyond the existing schema 4 is required.

Tests cover documented layouts, internal identity-store links, root escapes,
shims, missing/foreign versions, bounded scans, stale approvals and version
mismatch. Disposable copies of real installed Node/Bun/Python are probed through
the app and CLI. Fake managers/hooks remain unexecuted; these are layout fixtures,
not a claim of testing installed mise/uv releases or every distribution variant.

Phase 4 implementation is complete. Cross-platform and release acceptance stays
explicit in [release acceptance](../validation/RELEASE-ACCEPTANCE.md); completion
does not turn unperformed checks into passes. Usage and local evidence are in
[PHASE-4](../PHASE-4.md) and [completion validation](../validation/PHASE-4-COMPLETION.md).

Storage references: [mise directories](https://mise.jdx.dev/directories.html),
[mise identity layout](https://mise.jdx.dev/dev-tools/install-layout),
[uv Python storage](https://docs.astral.sh/uv/reference/storage/) and
[uv interpreter keys](https://docs.astral.sh/uv/concepts/python-versions/).
